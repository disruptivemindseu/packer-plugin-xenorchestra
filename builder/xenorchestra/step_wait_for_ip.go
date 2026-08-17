package xenorchestra

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepWaitForIP struct{}

func (s *StepWaitForIP) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	client := state.Get("client").(*Client)

	vmID, ok := state.GetOk("vm_id")
	if !ok {
		ui.Error("VM ID not found in state")
		return multistep.ActionHalt
	}

	timeout := 15 * time.Minute

	ui.Say(fmt.Sprintf("Waiting for VM to get IP address (timeout: %v)...", timeout))

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	deadline := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			ui.Say("Wait for IP interrupted")
			return multistep.ActionHalt
		case <-deadline:
			ui.Error("Timeout waiting for VM IP address")
			return multistep.ActionHalt
		case <-ticker.C:
			vm, err := client.GetVM(vmID.(string))
			if err != nil {
				ui.Say(fmt.Sprintf("Error getting VM status: %v (retrying...)", err))
				continue
			}

			// Look for IPv4 address
			ipv4 := extractIPv4(vm.Addresses)
			if ipv4 != "" {
				ui.Say(fmt.Sprintf("VM got IP address: %s", ipv4))
				state.Put("vm_ip", ipv4)
				return multistep.ActionContinue
			}

			ui.Say("VM IP not yet assigned (retrying in 5s)...")
		}
	}
}

func (s *StepWaitForIP) Cleanup(state multistep.StateBag) {
	// No cleanup needed
}

func extractIPv4(addresses map[string]string) string {
	// Look for keys like "0/ipv4/0", "0/ipv4/1", etc.
	for key, value := range addresses {
		if strings.Contains(key, "/ipv4/") {
			return value
		}
	}
	return ""
}
