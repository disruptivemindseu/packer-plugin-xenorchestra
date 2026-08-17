package xenorchestra

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepFindISO struct{}

func (s *StepFindISO) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)
	client := state.Get("client").(*Client)

	ui.Say(fmt.Sprintf("Finding ISO: %s", config.ISOName))

	// Get all SRs to find the ISO SR
	srs, err := client.GetSRs()
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to get storage repositories: %v", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	var isoSRID string
	var availableSRs []string
	for _, sr := range srs {
		availableSRs = append(availableSRs, sr.NameLabel)
		if sr.NameLabel == config.SRISOName {
			isoSRID = sr.ID
			break
		}
	}

	if isoSRID == "" {
		errMsg := fmt.Sprintf("ISO storage repository '%s' not found. Available SRs: %v", config.SRISOName, availableSRs)
		ui.Error(errMsg)
		state.Put("error", fmt.Errorf("%s", errMsg))
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Found ISO SR: %s (%s)", config.SRISOName, isoSRID))

	// Get all VDIs to find the ISO
	vdis, err := client.GetVDIs()
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to get VDIs: %v", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	var isoVDIID string
	for _, vdi := range vdis {
		if vdi.NameLabel == config.ISOName && vdi.SR == isoSRID {
			isoVDIID = vdi.ID
			break
		}
	}

	if isoVDIID == "" {
		err := fmt.Errorf("ISO '%s' not found in SR '%s'", config.ISOName, config.SRISOName)
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Found ISO: %s (%s)", config.ISOName, isoVDIID))

	state.Put("iso_vdi_id", isoVDIID)
	state.Put("iso_sr_id", isoSRID)

	return multistep.ActionContinue
}

func (s *StepFindISO) Cleanup(state multistep.StateBag) {
	// No cleanup needed
}
