package xenorchestra

import (
	"context"
	"fmt"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

const BuilderId = "xenorchestra"

type Config struct {
	// API Connection
	RestEndpoint string `mapstructure:"rest_endpoint"`
	Token        string `mapstructure:"token"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`

	// Infrastructure
	PoolName     string   `mapstructure:"pool_name"`
	SRName       string   `mapstructure:"sr_name"`
	SRISOName    string   `mapstructure:"sr_iso_name"`
	NetworkNames []string `mapstructure:"network_names"`
	TemplateName string   `mapstructure:"template_name"`

	// ISO
	ISOName string `mapstructure:"iso_name"`

	// VM Configuration
	VMName        string `mapstructure:"vm_name"`
	VMDescription string `mapstructure:"vm_description"`
	VCPUs      int    `mapstructure:"vcpus"`
	VMMemory      int64  `mapstructure:"vm_memory"`
	DiskSize      int64  `mapstructure:"disk_size"`
	DiskName      string `mapstructure:"disk_name"`
	Firmware      string `mapstructure:"firmware"`

	// Boot & Installation
	HTTPDirectory string   `mapstructure:"http_directory"`
	HTTPPort      int      `mapstructure:"http_port"`
	BootWait      string   `mapstructure:"boot_wait"`
	BootCommand   []string `mapstructure:"boot_command"`
	Mode          string   `mapstructure:"mode"`

	// Output
	KeepVM string `mapstructure:"keep_vm"`
}

type Builder struct {
	config Config
	client *Client
}

func (b *Builder) ConfigSpec() hcldec.ObjectSpec {
	return (&Config{}).FlattenedHCL2Spec()
}

func (b *Builder) Prepare(raws ...interface{}) ([]string, []string, error) {
	for _, raw := range raws {
		if ctyVal, ok := raw.(cty.Value); ok {
			// cty.Value from HCL2 - convert to FlatConfig
			var fc FlatConfig
			if err := gocty.FromCtyValue(ctyVal, &fc); err != nil {
				return nil, nil, fmt.Errorf("failed to decode config: %w", err)
			}
			// Copy FlatConfig pointers to Config
			if fc.RestEndpoint != nil {
				b.config.RestEndpoint = *fc.RestEndpoint
			}
			if fc.Token != nil {
				b.config.Token = *fc.Token
			}
			if fc.PoolName != nil {
				b.config.PoolName = *fc.PoolName
			}
			if fc.SRName != nil {
				b.config.SRName = *fc.SRName
			}
			if fc.SRISOName != nil {
				b.config.SRISOName = *fc.SRISOName
			}
			if fc.NetworkNames != nil {
				b.config.NetworkNames = *fc.NetworkNames
			}
			if fc.TemplateName != nil {
				b.config.TemplateName = *fc.TemplateName
			}
			if fc.ISOName != nil {
				b.config.ISOName = *fc.ISOName
			}
			if fc.VMName != nil {
				b.config.VMName = *fc.VMName
			}
			if fc.VMDescription != nil {
				b.config.VMDescription = *fc.VMDescription
			}
			if fc.VCPUs != nil {
				b.config.VCPUs = *fc.VCPUs
			}
			if fc.VMMemory != nil {
				b.config.VMMemory = *fc.VMMemory
			}
			if fc.DiskSize != nil {
				b.config.DiskSize = *fc.DiskSize
			}
			if fc.DiskName != nil {
				b.config.DiskName = *fc.DiskName
			}
			if fc.Firmware != nil {
				b.config.Firmware = *fc.Firmware
			}
			if fc.HTTPDirectory != nil {
				b.config.HTTPDirectory = *fc.HTTPDirectory
			}
			if fc.HTTPPort != nil {
				b.config.HTTPPort = int(*fc.HTTPPort)
			}
			if fc.BootWait != nil {
				b.config.BootWait = *fc.BootWait
			}
			if fc.BootCommand != nil {
				b.config.BootCommand = *fc.BootCommand
			}
			if fc.Mode != nil {
				b.config.Mode = *fc.Mode
			}
			if fc.KeepVM != nil {
				b.config.KeepVM = *fc.KeepVM
			}
			if fc.Username != nil {
				b.config.Username = *fc.Username
			}
			if fc.Password != nil {
				b.config.Password = *fc.Password
			}
		}
	}

	// Set defaults
	if b.config.VCPUs == 0 {
		b.config.VCPUs = 2
	}
	if b.config.VMMemory == 0 {
		b.config.VMMemory = 4096 * 1024 * 1024
	}
	if b.config.Firmware == "" {
		b.config.Firmware = "bios"
	}
	if b.config.HTTPPort == 0 {
		b.config.HTTPPort = 8000
	}
	if b.config.Mode == "" {
		b.config.Mode = "tools"
	}

	// Validate required fields
	var errs []error
	if b.config.RestEndpoint == "" {
		errs = append(errs, fmt.Errorf("rest_endpoint is required"))
	}
	if b.config.Token == "" {
		errs = append(errs, fmt.Errorf("token is required"))
	}
	if b.config.PoolName == "" {
		errs = append(errs, fmt.Errorf("pool_name is required"))
	}
	if b.config.SRName == "" {
		errs = append(errs, fmt.Errorf("sr_name is required"))
	}
	if b.config.ISOName == "" {
		errs = append(errs, fmt.Errorf("iso_name is required"))
	}
	if b.config.TemplateName == "" {
		errs = append(errs, fmt.Errorf("template_name is required"))
	}
	if b.config.KeepVM != "" && b.config.KeepVM != "on_success" {
		errs = append(errs, fmt.Errorf("keep_vm must be \"on_success\" or unset, got %q", b.config.KeepVM))
	}

	if len(errs) > 0 {
		return nil, nil, fmt.Errorf("config errors: %v", errs)
	}

	// Initialize API client
	var err error
	b.client, err = NewClient(b.config.RestEndpoint, b.config.Token)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	return nil, nil, nil
}

func (b *Builder) Run(ctx context.Context, ui packer.Ui, hook packer.Hook) (packer.Artifact, error) {
	steps := []multistep.Step{
		&StepHTTPServer{},
		&StepFindISO{},
		&StepCreateVM{},
		&StepStartVM{},
		&StepBootVM{},
	}

	// Add mode-specific steps
	if b.config.Mode == "shutdown" {
		// Preseed will trigger shutdown automatically
		steps = append(steps, &StepWaitForShutdown{})
	} else {
		// Wait for IP, then manually shutdown
		steps = append(steps, &StepWaitForIP{})
		steps = append(steps, &StepShutdown{})
	}

	// Convert to template
	steps = append(steps, &StepConvertTemplate{})

	state := new(multistep.BasicStateBag)
	state.Put("ui", ui)
	state.Put("hook", hook)
	state.Put("config", &b.config)
	state.Put("client", b.client)

	ui.Say("Building Xen Orchestra VM...")

	runner := &multistep.BasicRunner{Steps: steps}
	runner.Run(ctx, state)

	if rawErr, ok := state.GetOk("error"); ok {
		return nil, rawErr.(error)
	}

	artifact := &Artifact{
		StateData: make(map[string]interface{}),
	}

	if vmID, ok := state.GetOk("vm_id"); ok {
		artifact.StateData["vm_id"] = vmID
	}

	return artifact, nil
}
