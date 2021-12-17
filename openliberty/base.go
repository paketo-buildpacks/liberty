package openliberty

import (
	"fmt"
	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/sherpa"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Base struct {
	BuildpackPath    string
	LayerContributor libpak.LayerContributor
	Logger           bard.Logger
}

func NewBase(buildpackPath string) Base {
	contributor := libpak.NewLayerContributor("config", map[string]interface{}{}, libcnb.LayerTypes{
		Launch: true,
	})
	return Base{
		BuildpackPath:    buildpackPath,
		LayerContributor: contributor,
	}
}

func (b Base) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	b.LayerContributor.Logger = b.Logger

	return b.LayerContributor.Contribute(layer, func() (libcnb.Layer, error) {
		if err := b.ContributeConfigTemplates(layer); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to contribute config templates\n%w", err)
		}

		layer.LaunchEnvironment.Default("BPI_OL_BASE_ROOT", layer.Path)

		return layer, nil
	})
}

func (b Base) ContributeConfigTemplates(layer libcnb.Layer) error {
	b.Logger.Header("Contributing config templates")
	// Create config templates directory
	templateDir := filepath.Join(layer.Path, "templates")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return fmt.Errorf("unable to create config template directory '%v'\n%w", templateDir, err)
	}

	srcDir := filepath.Join(b.BuildpackPath, "templates")
	entries, err := ioutil.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("unable to read source directory\n%w", err)
	}

	for _, entry := range entries {
		err := func() error {
			srcPath := filepath.Join(srcDir, entry.Name())
			b.Logger.Bodyf("Copying %v", srcPath)
			destPath := filepath.Join(templateDir, entry.Name())
			in, err := os.Open(srcPath)
			if err != nil {
				return fmt.Errorf("unable to open source template file '%v'\n%w", srcPath, err)
			}
			defer in.Close()
			err = sherpa.CopyFile(in, destPath)
			if err != nil {
				return fmt.Errorf("unable to copy template from '%v' -> '%v'\n%w", srcPath, destPath, err)
			}
			return nil
		}()

		if err != nil {
			return err
		}
	}
	return nil
}

func (Base) Name() string {
	return "base"
}
