package xenorchestra

import (
	"context"
	"fmt"
	"strings"

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
	pools, err := client.GetPools()
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to get pools: %v", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	var poolID string
	for _, pool := range pools {
		if pool.NameLabel == config.PoolName {
			poolID = pool.ID
			break
		}
	}

	if poolID == "" {
		err := fmt.Errorf("pool '%s' not found", config.PoolName)
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	// Find the template
	templates, err := client.GetTemplates()
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to get templates: %v", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	var templateRef string
	for _, tmpl := range templates {
		if tmpl.NameLabel == config.TemplateName {
			// API expects just the template UUID part (after the pool ID prefix)
			// Template ID format: poolid-uuid where poolid is 5 dash-separated parts
			parts := strings.Split(tmpl.ID, "-")
			if len(parts) >= 10 {
				// Template UUID is parts 5-9 (the last 5 parts after pool ID)
				templateRef = strings.Join(parts[5:], "-")
			} else {
				templateRef = tmpl.ID
			}
			break
		}
	}

	if templateRef == "" {
		err := fmt.Errorf("template '%s' not found", config.TemplateName)
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Using template: %s", config.TemplateName))

	isoVDIID := state.Get("iso_vdi_id").(string)

	// Find the SR where to create the disk
	srs, err := client.GetSRs()
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to get storage repositories: %v", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	var srID string
	for _, sr := range srs {
		if sr.NameLabel == config.SRName {
			srID = sr.ID
			break
		}
	}

	if srID == "" {
		err := fmt.Errorf("storage repository '%s' not found", config.SRName)
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Using storage repository: %s", config.SRName))

	// Build VDIs array - create primary disk
	// disk_size in config is in MB, convert to bytes
	diskSizeBytes := config.DiskSize * 1024 * 1024
	vdis := []map[string]interface{}{
		{
			"name_label": config.DiskName,
			"sr":         srID,
			"size":       diskSizeBytes,
		},
	}

	// Build VIFs array - attach to first network if specified
	var vifs []map[string]interface{}
	if len(config.NetworkNames) > 0 {
		ui.Say(fmt.Sprintf("Looking up %d network(s): %v", len(config.NetworkNames), config.NetworkNames))

		networks, err := client.GetNetworks()
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to get networks: %v", err))
			state.Put("error", err)
			return multistep.ActionHalt
		}

		for i, networkName := range config.NetworkNames {
			var networkID string
			for _, net := range networks {
				if net.NameLabel == networkName {
					networkID = net.ID
					break
				}
			}

			if networkID == "" {
				ui.Say(fmt.Sprintf("Warning: network '%s' not found, skipping", networkName))
				continue
			}

			ui.Say(fmt.Sprintf("Attaching network: %s", networkName))
			vifs = append(vifs, map[string]interface{}{
				"network": networkID,
				"device":  fmt.Sprintf("%d", i),
			})
		}
	} else {
		ui.Say("No networks configured (network_names is empty)")
	}

	// Create VM from template with ISO boot
	vmConfig := map[string]interface{}{
		"name_label":       config.VMName,
		"name_description": config.VMDescription,
		"template":         templateRef,
		"memory":           config.VMMemory,
		"cpus":             config.VCPUs,
		"hvmBootFirmware":  config.Firmware,
		"boot":             true,
		"vdis":             vdis,
		"install": map[string]interface{}{
			"method":     "cdrom",
			"repository": isoVDIID,
		},
	}

	// Add VIFs if we have any
	if len(vifs) > 0 {
		vmConfig["vifs"] = vifs
	}

	vmID, err := client.CreateVM(poolID, vmConfig)
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
		vmIDStr := vmID.(string)
		ui.Say(fmt.Sprintf("Force shutting down VM: %s", vmIDStr))
		if err := client.HardShutdownVM(vmIDStr); err != nil {
			ui.Say(fmt.Sprintf("Hard shutdown error (may already be off): %v", err))
		} else {
			ui.Say("VM shut down")
		}
	} else {
		ui.Say("Keeping VM (build completed successfully with keep_vm=on_success)")
	}
}
