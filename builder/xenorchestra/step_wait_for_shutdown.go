package xenorchestra

import (
	"context"
	"fmt"
	"time"

	"github.com/gofrs/uuid"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepWaitForShutdown struct{}

func (s *StepWaitForShutdown) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	client := state.Get("client").(*Client)

	vmID, ok := state.GetOk("vm_id")
	if !ok {
		ui.Error("VM ID not found in state")
		return multistep.ActionHalt
	}

	timeout := 30 * time.Minute

	ui.Say(fmt.Sprintf("Waiting for VM to shutdown (timeout: %v)...", timeout))

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	deadline := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			ui.Say("Wait for shutdown interrupted")
			return multistep.ActionHalt
		case <-deadline:
			ui.Error("Timeout waiting for VM to shutdown")
			return multistep.ActionHalt
		case <-ticker.C:
			vm, err := client.VM().GetByID(ctx, vmID.(uuid.UUID))
			if err != nil {
				ui.Say(fmt.Sprintf("Error getting VM status: %v (retrying...)", err))
				continue
			}

			// Check if VM is halted
			if vm.PowerState == payloads.PowerStateHalted {
				ui.Say("VM has shut down")
				return multistep.ActionContinue
			}

			ui.Say(fmt.Sprintf("VM is still running (state: %s, retrying in 10s)...", vm.PowerState))
		}
	}
}

func (s *StepWaitForShutdown) Cleanup(state multistep.StateBag) {
	// No cleanup needed
}
