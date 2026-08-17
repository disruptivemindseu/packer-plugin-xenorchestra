package xenorchestra

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepShutdown struct{}

func (s *StepShutdown) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	client := state.Get("client").(*Client)
	vmID := state.Get("vm_id").(string)

	ui.Say(fmt.Sprintf("Shutting down VM: %s", vmID))

	if err := client.ShutdownVM(vmID); err != nil {
		ui.Error(fmt.Sprintf("Failed to shutdown VM: %v", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say("VM shutdown successfully")

	return multistep.ActionContinue
}

func (s *StepShutdown) Cleanup(state multistep.StateBag) {
	// No cleanup needed
}
