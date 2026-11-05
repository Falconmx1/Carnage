package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"golang.org/x/net/proxy"
)

// BuildChain construye un Dialer que pasa por una lista de proxies SOCKS5.
// El orden es: el primero es el más cercano al Admin, el último es el más cercano al destino.
func BuildChain(proxies []string) (proxy.Dialer, error) {
	if len(proxies) == 0 {
		return proxy.Direct, nil
	}

	first, err := proxy.SOCKS5("tcp", proxies[0], nil, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("error creando dialer para %s: %w", proxies[0], err)
	}

	current := first
	for i := 1; i < len(proxies); i++ {
		next, err := proxy.SOCKS5("tcp", proxies[i], nil, current)
		if err != nil {
			return nil, fmt.Errorf("error creando dialer para %s: %w", proxies[i], err)
		}
		current = next
	}

	return current, nil
}

// DialThroughChain conecta a un destino a través de una cadena de proxies.
func DialThroughChain(chain proxy.Dialer, target string) (net.Conn, error) {
	return chain.Dial("tcp", target)
}

// LocalForwarder acepta conexiones locales y las redirige a través de la cadena
// hacia el destino final. Es bloqueante.
func LocalForwarder(ctx context.Context, listenAddr string, chain proxy.Dialer, target string) error {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("error escuchando en %s: %w", listenAddr, err)
	}
	defer ln.Close()
	fmt.Printf("[+] Forwarder local: %s -> %s (a través de cadena)\n", listenAddr, target)

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
		go func(c net.Conn) {
			defer c.Close()
			remote, err := chain.Dial("tcp", target)
			if err != nil {
				fmt.Printf("[-] Error conectando a %s vía cadena: %v\n", target, err)
				return
			}
			defer remote.Close()

			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); io.Copy(remote, c) }()
			go func() { defer wg.Done(); io.Copy(c, remote) }()
			wg.Wait()
		}(conn)
	}
}
