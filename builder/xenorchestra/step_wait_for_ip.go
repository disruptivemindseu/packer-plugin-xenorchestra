package xenorchestra

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/gofrs/uuid"
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
			vm, err := client.VM().GetByID(ctx, vmID.(uuid.UUID))
			if err != nil {
				ui.Say(fmt.Sprintf("Error getting VM status: %v (retrying...)", err))
				continue
			}

			if ip := net.ParseIP(vm.MainIpAddress); ip.To4() != nil {
				ui.Say(fmt.Sprintf("VM got IP address: %s", vm.MainIpAddress))
				state.Put("vm_ip", vm.MainIpAddress)
				return multistep.ActionContinue
			}

			ui.Say("VM IP not yet assigned (retrying in 5s)...")
		}
	}
}

func (s *StepWaitForIP) Cleanup(state multistep.StateBag) {
	// No cleanup needed
}
