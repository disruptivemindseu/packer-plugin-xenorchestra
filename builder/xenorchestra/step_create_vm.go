package xenorchestra

import (
	"context"
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepCreateVM struct{}

func (s *StepCreateVM) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)
	client := state.Get("client").(*Client)

	ui.Say(fmt.Sprintf("Creating VM: %s", config.VMName))

	// Get pool ID
	pools, err := client.Pool().GetAll(ctx, 0, byName(config.PoolName))
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to get pools: %v", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	var poolID uuid.UUID
	for _, pool := range pools {
		if pool.NameLabel == config.PoolName {
			poolID = pool.ID
			break
		}
	}

	if poolID == uuid.Nil {
		err := fmt.Errorf("pool '%s' not found", config.PoolName)
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	templateID, err := client.FindTemplate(ctx, config.TemplateName)
	if err != nil {
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Using template: %s", config.TemplateName))

	isoVDIID := state.Get("iso_vdi_id").(uuid.UUID)

	// Find the SR where to create the disk
	srs, err := client.SR().GetAll(ctx, 0, byName(config.SRName))
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to get storage repositories: %v", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	var srID uuid.UUID
	for _, sr := range srs {
		if sr.NameLabel == config.SRName {
			srID = sr.ID
			break
		}
	}

	if srID == uuid.Nil {
		err := fmt.Errorf("storage repository '%s' not found", config.SRName)
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Using storage repository: %s", config.SRName))

	// Build VDIs array - create primary disk
	// disk_size in config is in MB, convert to bytes
	diskSizeBytes := config.DiskSize * 1024 * 1024
	vdis := []payloads.VDIParams{
		{
			NameLabel: &config.DiskName,
			SR:        &srID,
			Size:      &diskSizeBytes,
		},
	}

	// Build VIFs array
	var vifs []payloads.VIFParams
	if len(config.NetworkNames) > 0 {
		ui.Say(fmt.Sprintf("Looking up %d network(s): %v", len(config.NetworkNames), config.NetworkNames))

		networks, err := client.Network().GetAll(ctx, 0, "")
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to get networks: %v", err))
			state.Put("error", err)
			return multistep.ActionHalt
		}

		for i, networkName := range config.NetworkNames {
			var networkID uuid.UUID
			for _, net := range networks {
				if net.NameLabel == networkName {
					networkID = net.ID
					break
				}
			}

			if networkID == uuid.Nil {
				ui.Say(fmt.Sprintf("Warning: network '%s' not found, skipping", networkName))
				continue
			}

			ui.Say(fmt.Sprintf("Attaching network: %s", networkName))
			device := payloads.StringifiedInt(i)
			vifs = append(vifs, payloads.VIFParams{
				Network: &networkID,
				Device:  &device,
			})
		}
	} else {
		ui.Say("No networks configured (network_names is empty)")
	}

	// Create VM from template with ISO boot
	memory := int(config.VMMemory)
	boot := true
	vmConfig := payloads.CreateVMParams{
		NameLabel:       config.VMName,
		NameDescription: config.VMDescription,
		Template:        templateID,
		Memory:          &memory,
		Boot:            &boot,
		VDIs:            vdis,
		VIFs:            vifs,
		Install: &payloads.InstallParams{
			Method:     payloads.InstallMethodCdrom,
			Repository: isoVDIID.String(),
		},
	}

	vmID, err := client.CreateVM(ctx, poolID, vmConfig, config.VCPUs, config.Firmware)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to create VM: %v", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Created VM: %s (%s)", config.VMName, vmID))
	state.Put("vm_id", vmID)

	return multistep.ActionContinue
}

func (s *StepCreateVM) Cleanup(state multistep.StateBag) {
	vmID, ok := state.GetOk("vm_id")
	if !ok {
		return
	}

	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(*Config)
	client := state.Get("client").(*Client)

	// Delete VM unless build completed successfully AND keep_vm allows it
	buildComplete, _ := state.GetOk("build_complete")
	shouldKeep := buildComplete != nil && config.KeepVM == "on_success"

	if !shouldKeep {
		ui.Say(fmt.Sprintf("Force shutting down VM: %s", vmID))
		if _, err := client.VM().HardShutdown(context.Background(), vmID.(uuid.UUID)); err != nil {
			ui.Say(fmt.Sprintf("Hard shutdown error (may already be off): %v", err))
		} else {
			ui.Say("VM shut down")
		}
	} else {
		ui.Say("Keeping VM (build completed successfully with keep_vm=on_success)")
	}
}
