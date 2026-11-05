package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version se inyecta en tiempo de compilación con -ldflags "-X main.Version=x.y.z"
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:     "carnage",
	Short:   "Carnage - Proxy multi-salto SOCKS5 + port forwarding + port reuse",
	Long: `Carnage es un framework de pentesting con modo Admin/Agent,
proxy SOCKS5 multi-salto, port forwarding, port reuse (SO_REUSEPORT),
comunicaciones TLS 1.3 + AES-GCM y compilación estática multiplataforma.

Uso exclusivo para pruebas de penetración autorizadas.`,
	Version: Version,
}

func init() {
	// ----- Flags de admin -----
	adminConnectCmd.Flags().StringVar(&adminAgentIP, "agent-ip", "127.0.0.1", "IP del Agent")
	adminConnectCmd.Flags().IntVar(&adminAgentPort, "agent-port", 443, "Puerto del Agent")

	adminListenCmd.Flags().StringVar(&adminListenAddr, "listen", "0.0.0.0:443", "Dirección de escucha del Admin")

	adminChainCmd.Flags().StringVar(&adminChainNodes, "nodes", "", "Lista de nodos SOCKS5 separados por coma (ej. a:1080,b:1080)")
	adminChainCmd.Flags().StringVar(&adminListenAddr, "listen", "127.0.0.1:1080", "Dirección local para el forwarder de la cadena")
	adminChainCmd.Flags().StringVar(&adminTarget, "target", "", "Destino final a través de la cadena (ej. 10.0.0.5:3389)")

	adminForwardCmd.Flags().IntVar(&adminListenPort, "listen-port", 13389, "Puerto local para el port forwarding")
	adminForwardCmd.Flags().StringVar(&adminTarget, "target", "", "Destino interno (ej. 127.0.0.1:3389)")

	adminReuseCmd.Flags().StringVar(&adminReuseAddr, "reuse-addr", "0.0.0.0:80", "Dirección del puerto ya abierto a reutilizar")
	adminReuseCmd.Flags().StringVar(&adminTarget, "target", "", "Destino interno (ej. 127.0.0.1:3389)")

	// ----- Flags de agent -----
	agentListenCmd.Flags().IntVar(&agentListenPort, "port", 443, "Puerto de escucha del Agent")
	agentConnectCmd.Flags().StringVar(&agentAdminIP, "admin-ip", "127.0.0.1", "IP del Admin")
	agentConnectCmd.Flags().IntVar(&agentAdminPort, "admin-port", 443, "Puerto del Admin")

	// ----- Registro de comandos -----
	adminCmd.AddCommand(adminConnectCmd, adminListenCmd, adminChainCmd, adminForwardCmd, adminReuseCmd)
	agentCmd.AddCommand(agentListenCmd, agentConnectCmd)
	rootCmd.AddCommand(adminCmd, agentCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
