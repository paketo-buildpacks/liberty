package openliberty

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/crush"
	"github.com/paketo-buildpacks/libpak/effect"
)

type Distribution struct {
	ApplicationPath  string
	LayerContributor libpak.DependencyLayerContributor
	Logger           bard.Logger
}

func NewDistribution(dependency libpak.BuildpackDependency, cache libpak.DependencyCache, applicationPath string) (Distribution, libcnb.BOMEntry) {
	contributor, entry := libpak.NewDependencyLayer(dependency, cache, libcnb.LayerTypes{
		Cache:  true,
		Launch: true,
	})
	return Distribution{
		ApplicationPath:  applicationPath,
		LayerContributor: contributor,
	}, entry
}

func (d Distribution) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	d.LayerContributor.Logger = d.Logger

	return d.LayerContributor.Contribute(layer, func(artifact *os.File) (libcnb.Layer, error) {
		d.Logger.Bodyf("Expanding to %s", layer.Path)
		if err := crush.ExtractZip(artifact, layer.Path, 1); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to expand Liberty Runtime\n%w", err)
		}

		outErrWriter := d.Logger.InfoWriter()
		if outErrWriter == nil {
			outErrWriter = ioutil.Discard
		}

		executor := effect.NewExecutor()
		if err := executor.Execute(effect.Execution{
			Command: filepath.Join(layer.Path, "bin", "server"),
			Args:    []string{"create", "defaultServer"},
			Dir:     layer.Path,
			Stdout:  outErrWriter,
			Stderr:  outErrWriter,
		}); err != nil {
			return libcnb.Layer{}, fmt.Errorf("could not create default server\n%w", err)
		}

		// these are used by the exec.d helper to successfully create the symlink to the dropin app
		layer.LaunchEnvironment.Default("BPI_OL_DROPIN_DIR", d.ApplicationPath)
		layer.LaunchEnvironment.Default("BPI_OL_RUNTIME_ROOT", layer.Path)

		// set logging to write to the console. Using `server run` instead of `server start` ensures that
		// stdout/stderr are actually written to their respective streams instead of to `console.log`
		layer.LaunchEnvironment.Override("WLP_LOGGING_MESSAGE_SOURCE", "")
		layer.LaunchEnvironment.Default("WLP_LOGGING_CONSOLE_SOURCE", "message,trace,accessLog,ffdc,audit")

		return layer, nil
	})
}

func (d Distribution) Name() string {
	return d.LayerContributor.LayerName()
}
