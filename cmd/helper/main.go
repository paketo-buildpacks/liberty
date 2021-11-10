package main

import (
	"log"
	"os"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/sherpa"
	"github.com/paketo-buildpacks/open-liberty/helper"
)

func main() {
	bindingPath := ""
	ok := false
	if bindingPath, ok = os.LookupEnv(libcnb.EnvServiceBindings); !ok {
		bindingPath = "/platform/bindings"
	}

	b, err := libcnb.NewBindingsFromPath(bindingPath)
	if err != nil {
		log.Fatal(err)
	}
	sherpa.Execute(func() error {
		return sherpa.Helpers(map[string]sherpa.ExecD{
			"linker": helper.FileLinker{Bindings: b, Logger: bard.NewLogger(os.Stdout)},
		})
	})
}
