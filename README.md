# Packer Plugin for Xen Orchestra

Status: Proof of Concept

Automates golden-image VM creation on Xen Orchestra by:
1. Creating a VM via REST API
2. Injecting Debian installer boot commands via VNC (WebSocket console)
3. Serving preseed.cfg over HTTP for unattended installation
4. Detecting when installation is complete (VM gets IP or shuts down)
5. Converting the VM to a template (Not yet implemented, missing end point)

## What Works 

- VM Creation — disk, network, CPU, memory, firmware (BIOS/UEFI) configuration
- Debian Installation — VNC boot command injection for automated installer
- Preseed Serving — local HTTP server delivers preseed.cfg to installer
- Installation Detection — two modes:
  - `mode = "tools"` (default): waits for VM to get an IP address
  - `mode = "shutdown"`: waits for VM to shut down (preseed-triggered)

### Not Implemented / Tested

- SSH provisioning or post-install scripting
- VM/template export
- Windows support
- Many features are probably missing compared to others providers. Please don't hesitate to make issues !

## Usage

See example directory

Requires `example/http/preseed.cfg` and .env configured
```bash
packer build example/debian13-uefi.pkr.hcl
```

## Build & Test

```bash
make build && make dev # Build and install plugin
go test ./builder/xenorchestra/ # Run unit tests
source .env && packer build example/debian13-uefi.pkr.hcl
```

## Why a POC?

This plugin demonstrates the feasibility of Packer on XO via REST API but requires additional work for production:
- Credential handling hardening (no plaintext passwords in logs)
- Error recovery and retry logic
- Support for additional Linux distributions & Windows
- Post-install provisioning strategy 
- Missing some endpoint in Xen Orchestra to have a full converage

Big thanks to the Vates Team, and Pierre B. who helped me figure out the console part.

Made with love, some Packer/Xen Orchestra/XCP-ng experience, and coffee. And Claude.