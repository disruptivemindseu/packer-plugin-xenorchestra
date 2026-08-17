package xenorchestra

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepHTTPServer struct{}

func (s *StepHTTPServer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ui := state.Get("ui").(packer.Ui)

	var httpPort uint = 0

	if config.HTTPDirectory == "" {
		// No HTTP server needed
		state.Put("http_port", int(httpPort))
		state.Put("http_ip", "")
		return multistep.ActionContinue
	}

	// Listen on configured port
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", config.HTTPPort))
	if err != nil {
		ui.Error(fmt.Sprintf("Error: unable to listen on port %d: %v", config.HTTPPort, err))
		return multistep.ActionHalt
	}

	httpPort = uint(config.HTTPPort)
	ui.Say(fmt.Sprintf("Starting HTTP server on port %d for %s", httpPort, config.HTTPDirectory))

	// Start HTTP file server in background
	fileServer := http.FileServer(http.Dir(config.HTTPDirectory))
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: fileServer,
	}

	go func() {
		server.Serve(listener)
	}()

	// Store port and listener for cleanup
	state.Put("http_port", int(httpPort))
	state.Put("http_listener", listener)
	state.Put("http_server", server)

	// Detect local IP by connecting to the XO endpoint
	httpIP := "127.0.0.1"

	parsedURL, err := url.Parse(config.RestEndpoint)
	if err == nil && parsedURL.Hostname() != "" {
		ui.Say("Detecting local IP by connecting to XO endpoint...")

		// Determine port (default based on scheme if not explicit)
		port := parsedURL.Port()
		if port == "" {
			if parsedURL.Scheme == "http" {
				port = "80"
			} else {
				port = "443"
			}
		}

		// Try to connect to the XO endpoint to see what local IP we use (IPv4 only)
		dialTarget := net.JoinHostPort(parsedURL.Hostname(), port)
		conn, err := net.Dial("tcp4", dialTarget)
		if err == nil {
			defer conn.Close()
			if addr, ok := conn.LocalAddr().(*net.TCPAddr); ok {
				// Only use IPv4
				if ipv4 := addr.IP.To4(); ipv4 != nil {
					httpIP = ipv4.String()
					ui.Say(fmt.Sprintf("Detected local IPv4: %s", httpIP))
				} else {
					ui.Say(fmt.Sprintf("Warning: got IPv6, need IPv4 (will use 127.0.0.1)"))
				}
			}
		} else {
			ui.Say(fmt.Sprintf("Warning: could not detect local IP: %v (will use 127.0.0.1)", err))
		}
	}

	ui.Say(fmt.Sprintf("HTTP server will be accessible at http://%s:%d", httpIP, httpPort))
	state.Put("http_ip", httpIP)

	return multistep.ActionContinue
}

func (s *StepHTTPServer) Cleanup(state multistep.StateBag) {
	if listener, ok := state.GetOk("http_listener"); ok {
		listener.(net.Listener).Close()
	}
	if server, ok := state.GetOk("http_server"); ok {
		server.(*http.Server).Close()
	}
}
