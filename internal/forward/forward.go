package forward

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

// TCPForward redirige conexiones desde listenAddr hacia targetAddr.
// Es bloqueante; se detiene cuando el listener se cierra.
func TCPForward(listenAddr, targetAddr string) error {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("error escuchando en %s: %w", listenAddr, err)
	}
	defer ln.Close()
	fmt.Printf("[+] Port forwarding: %s -> %s\n", listenAddr, targetAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			return fmt.Errorf("error aceptando conexión: %w", err)
		}
		go handleForward(conn, targetAddr)
	}
}

func handleForward(src net.Conn, targetAddr string) {
	defer src.Close()

	dst, err := net.Dial("tcp", targetAddr)
	if err != nil {
		fmt.Printf("[-] Error conectando a %s: %v\n", targetAddr, err)
		return
	}
	defer dst.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(dst, src)
	}()
	go func() {
		defer wg.Done()
		io.Copy(src, dst)
	}()

	wg.Wait()
}

// ReuseListener crea un listener TCP que comparte el puerto con otros procesos
// usando SO_REUSEPORT. Útil para esconderse en puertos ya abiertos como el 80.
// Solo funciona en Linux/Unix. En otros sistemas devuelve un listener normal.
func ReuseListener(ctx context.Context, network, addr string) (net.Listener, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var sockErr error
			err := c.Control(func(fd uintptr) {
				// Intentar habilitar SO_REUSEPORT
				sockErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
				if sockErr != nil {
					// Si falla, intentar SO_REUSEADDR como fallback
					sockErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
				}
			})
			if err != nil {
				return err
			}
			return sockErr
		},
	}
	return lc.Listen(ctx, network, addr)
}

// ReuseForward redirige conexiones usando un listener con SO_REUSEPORT.
func ReuseForward(ctx context.Context, listenAddr, targetAddr string) error {
	ln, err := ReuseListener(ctx, "tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("error creando listener con reuse en %s: %w", listenAddr, err)
	}
	defer ln.Close()
	fmt.Printf("[+] Port reuse activo: %s -> %s\n", listenAddr, targetAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return fmt.Errorf("error aceptando conexión: %w", err)
			}
		}
		go handleForward(conn, targetAddr)
	}
}
