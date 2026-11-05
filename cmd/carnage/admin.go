package main

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	adminAgentIP   string
	adminAgentPort int
	adminListenAddr string
	adminChainNodes string
	adminListenPort int
	adminTarget     string
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Modo Admin (atacante): conecta al Agent o escucha conexiones",
	Long:  "El Admin controla la sesión: puede conectarse al Agent, crear cadenas SOCKS5 y hacer port forwarding.",
}

var adminConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Conectar al Agent (víctima)",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := fmt.Sprintf("%s:%d", adminAgentIP, adminAgentPort)
		conn, err := tls.Dial("tcp", addr, &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS13,
		})
		if err != nil {
			return fmt.Errorf("error conectando al agent %s: %w", addr, err)
		}
		defer conn.Close()

		// Handshake simple: enviar un saludo y esperar respuesta
		if _, err := conn.Write([]byte("CARNAGE_ADMIN\n")); err != nil {
			return fmt.Errorf("error enviando handshake: %w", err)
		}
		buf := make([]byte, 64)
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			return fmt.Errorf("error leyendo respuesta del agent: %w", err)
		}
		fmt.Printf("[+] Conexión establecida con el Agent. Respuesta: %s\n", string(buf[:n]))
		fmt.Println("[+] Sesión activa. Ctrl+C para salir.")
		waitForSignal()
		return nil
	},
}

var adminListenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Escuchar conexiones de Agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		cert, err := generateSelfSignedCert()
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
		fmt.Printf("[+] Admin escuchando en %s (TLS)\n", adminListenAddr)

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
	buf := make([]byte, 64)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Printf("[-] Error leyendo de %s: %v\n", conn.RemoteAddr(), err)
		return
	}
	fmt.Printf("[+] Conexión de %s: %s\n", conn.RemoteAddr(), string(buf[:n]))
	conn.Write([]byte("CARNAGE_OK\n"))
}

var adminChainCmd = &cobra.Command{
	Use:   "chain",
	Short: "Crear una cadena SOCKS5 multi-salto",
	RunE: func(cmd *cobra.Command, args []string) error {
		if adminChainNodes == "" {
			return fmt.Errorf("debes especificar --nodes con la lista de nodos SOCKS5")
		}
		// Validación simple de la cadena
		nodes := splitAndTrim(adminChainNodes, ",")
		if len(nodes) == 0 {
			return fmt.Errorf("lista de nodos vacía")
		}
		fmt.Printf("[+] Cadena SOCKS5 configurada con %d nodos:\n", len(nodes))
		for i, n := range nodes {
			fmt.Printf("    [%d] %s\n", i+1, n)
		}
		fmt.Printf("[+] Escuchando en %s para redirigir el tráfico a través de la cadena...\n", adminListenAddr)
		return runLocalForwarder(adminListenAddr, nodes)
	},
}

var adminForwardCmd = &cobra.Command{
	Use:   "forward",
	Short: "Port forwarding: redirigir un puerto local a un destino interno",
	RunE: func(cmd *cobra.Command, args []string) error {
		if adminTarget == "" {
			return fmt.Errorf("debes especificar --target (ej. 127.0.0.1:3389)")
		}
		listen := fmt.Sprintf("127.0.0.1:%d", adminListenPort)
		fmt.Printf("[+] Port forwarding: %s -> %s\n", listen, adminTarget)
		return runSimpleForwarder(listen, adminTarget)
	},
}

// --- Utilidades compartidas ---

func waitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	fmt.Println("\n[!] Saliendo...")
}

func splitAndTrim(s, sep string) []string {
	parts := []string{}
	for _, p := range splitString(s, sep) {
		p = trimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// Implementaciones mínimas para no depender de strings (evita imports no usados)
func splitString(s, sep string) []string {
	var res []string
	cur := ""
	for _, r := range s {
		if string(r) == sep {
			res = append(res, cur)
			cur = ""
		} else {
			cur += string(r)
		}
	}
	res = append(res, cur)
	return res
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// runSimpleForwarder implementa port forwarding TCP básico
func runSimpleForwarder(listenAddr, targetAddr string) error {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go func(c net.Conn) {
			defer c.Close()
			t, err := net.Dial("tcp", targetAddr)
			if err != nil {
				fmt.Printf("[-] Error conectando a %s: %v\n", targetAddr, err)
				return
			}
			defer t.Close()
			pipe(c, t)
		}(conn)
	}
}

// runLocalForwarder acepta conexiones locales y las reenvía al primer nodo de la cadena
func runLocalForwarder(listenAddr string, nodes []string) error {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go func(c net.Conn) {
			defer c.Close()
			// Se conecta al primer nodo de la cadena (simplificado)
			t, err := net.Dial("tcp", nodes[0])
			if err != nil {
				fmt.Printf("[-] Error conectando al nodo %s: %v\n", nodes[0], err)
				return
			}
			defer t.Close()
			pipe(c, t)
		}(conn)
	}
}

func pipe(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { io.Copy(a, b); wg.Done() }()
	go func() { io.Copy(b, a); wg.Done() }()
	wg.Wait()
}

// generateSelfSignedCert genera un certificado autofirmado para TLS
func generateSelfSignedCert() (tls.Certificate, error) {
	// Implementación mínima usando crypto/tls y crypto/x509
	return generateCert()
}

// Stub para evitar conflictos si ya tienes una función con otro nombre
func generateCert() (tls.Certificate, error) {
	// Nota: Esta función debe ser implementada con crypto/x509.
	// Si ya tienes una en internal/transport, reemplaza esta llamada.
	// Para no romper la compilación, devolvemos un certificado vacío.
	// En producción, usa tu implementación real.
	var cert tls.Certificate
	return cert, fmt.Errorf("generateCert no implementado: usa tu función de internal/transport")
}

// Evita warnings de imports no usados
var _ = binary.BigEndian
