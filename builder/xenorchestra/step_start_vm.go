package xenorchestra

import (
	"context"
	"fmt"
	"time"

	"github.com/gofrs/uuid"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepStartVM struct{}

func (s *StepStartVM) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	client := state.Get("client").(*Client)
	vmID := state.Get("vm_id").(uuid.UUID)

	ui.Say(fmt.Sprintf("Starting VM: %s", vmID))

	if _, err := client.VM().Start(ctx, vmID, nil); err != nil {
		ui.Error(fmt.Sprintf("Failed to start VM: %v", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say("VM start command sent, waiting for boot...")
	time.Sleep(5 * time.Second)

	return multistep.ActionContinue
}

func (s *StepStartVM) Cleanup(state multistep.StateBag) {
	// Cleanup not needed as we shutdown explicitly
}
