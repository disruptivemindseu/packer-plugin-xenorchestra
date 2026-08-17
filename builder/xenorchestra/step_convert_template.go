package xenorchestra

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// StepConvertTemplate marks the build complete. XO does not expose a "set as template" endpoint yet,
// so manual template creation via UI is required after the build.
type StepConvertTemplate struct{}

func (s *StepConvertTemplate) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	vmID := state.Get("vm_id").(string)

	ui.Say(fmt.Sprintf("Build complete. VM %s is ready.", vmID))
	ui.Say("Note: XO does not expose a 'set as template' endpoint. Convert the VM to a template manually via the Xen Orchestra UI.")

	// Mark build as complete - VM cleanup will respect keep_vm setting
	state.Put("build_complete", true)

	return multistep.ActionContinue
}

func (s *StepConvertTemplate) Cleanup(state multistep.StateBag) {
	// No cleanup needed
}
