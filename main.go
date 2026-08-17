package main

import (
	"fmt"
	"os"

	"github.com/disruptivemindseu/packer-plugin-xenorchestra/builder/xenorchestra"
	"github.com/hashicorp/packer-plugin-sdk/plugin"
	pluginVersion "github.com/hashicorp/packer-plugin-sdk/version"
)

var (
	Version           = "0.0.1"
	VersionPrerelease = ""
	VersionMetadata   = ""
)

func main() {
	packer := plugin.NewSet()
	packer.RegisterBuilder("iso", new(xenorchestra.Builder))
	packer.SetVersion(pluginVersion.NewPluginVersion(Version, VersionPrerelease, VersionMetadata))
	err := packer.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
