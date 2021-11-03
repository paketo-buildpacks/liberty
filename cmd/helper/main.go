package main

import (
	"os"

	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/sherpa"
	"github.com/paketo-buildpacks/open-liberty/helper"
)

func main() {
	sherpa.Execute(func() error {
		return sherpa.Helpers(map[string]sherpa.ExecD{
			"linker": helper.ApplicationLinker{Logger: bard.NewLogger(os.Stdout)},
		})
	})
}
