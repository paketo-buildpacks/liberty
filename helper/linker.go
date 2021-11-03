package helper

import (
	"os"
	"path/filepath"

	"github.com/paketo-buildpacks/libpak/bard"
)

type ApplicationLinker struct {
	Logger bard.Logger
}

func (a ApplicationLinker) Execute() (map[string]string, error) {
	appDir, ok := os.LookupEnv("BPI_OL_DROPIN_DIR")
	if !ok {
		appDir = "/workspace"
	}

	layerDir, ok := os.LookupEnv("BPI_OL_RUNTIME_ROOT")
	if !ok {
		layerDir = "/layers/paketo-buildpacks_open-liberty/open-liberty-runtime"
	}

	return nil, os.Symlink(appDir, filepath.Join(layerDir, "usr", "servers", "defaultServer", "dropins", filepath.Base(appDir)))
}
