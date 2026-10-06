package xenorchestra

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepBootVM struct{}

func (s *StepBootVM) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)
	vmID := state.Get("vm_id").(uuid.UUID).String()

	if len(config.BootCommand) == 0 {
		ui.Say("No boot commands - skipping boot automation")
		return multistep.ActionContinue
	}

	ui.Say(fmt.Sprintf("Connecting to VM console: %s", vmID))

	// Wait for boot_wait before trying to connect
	bootWaitDuration, err := time.ParseDuration(config.BootWait)
	if err != nil {
		bootWaitDuration = 15 * time.Second
	}
	ui.Say(fmt.Sprintf("Waiting for boot (%s)...", config.BootWait))
	time.Sleep(bootWaitDuration)

	// Extract host from endpoint URL properly
	parsedURL, err := url.Parse(config.RestEndpoint)
	if err != nil {
		err := fmt.Errorf("invalid rest_endpoint: %w", err)
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	host := parsedURL.Hostname()
	if host == "" {
		err := fmt.Errorf("invalid rest_endpoint: no host found in %s", config.RestEndpoint)
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	// Get username and password from config
	username := config.Username
	password := config.Password

	if username == "" || password == "" {
		err := fmt.Errorf("username and password are required for console access")
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	// Connect to WebSocket console with auth
	ui.Say("Signing in and connecting to console...")
	ws, err := ConnectWithAuth(config.RestEndpoint, username, password, vmID)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to connect to console: %v", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}
	defer ws.Close()

	// Initialize RFB client
	ui.Say("Initializing RFB client...")
	rfb, err := NewRFBClient(ws)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to initialize RFB: %v", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say("RFB handshake complete, sending boot commands")

	// Get HTTP server info for template substitution
	httpIP := ""
	httpPort := 0
	if ip, ok := state.GetOk("http_ip"); ok {
		httpIP = ip.(string)
	}
	if port, ok := state.GetOk("http_port"); ok {
		httpPort = port.(int)
	}

	// Send boot commands with template substitution
	for _, command := range config.BootCommand {
		// Simple template substitution
		command = strings.ReplaceAll(command, "{{.HTTPIP}}", httpIP)
		command = strings.ReplaceAll(command, "{{.HTTPPort}}", fmt.Sprintf("%d", httpPort))
		if len(command) == 0 {
			continue
		}

		ui.Say(fmt.Sprintf("Sending: %s", command))

		// Send command (handles <enter>, <tab>, <esc>, etc.)
		if err := rfb.SendCommand(command); err != nil {
			err := fmt.Errorf("error sending command: %w", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		time.Sleep(100 * time.Millisecond)
	}

	ui.Say("Boot commands completed - waiting for installation...")

	// Wait with context cancellation support
	select {
	case <-time.After(5 * time.Minute):
		// Normal timeout
	case <-ctx.Done():
		// Context cancelled (Ctrl+C)
		ui.Say("Installation interrupted")
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

func (s *StepBootVM) Cleanup(state multistep.StateBag) {
	// Nothing to clean up
}
