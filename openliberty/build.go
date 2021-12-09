package openliberty

import (
	"fmt"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
)

type Build struct {
	Logger bard.Logger
}

func (b Build) Build(context libcnb.BuildContext) (libcnb.BuildResult, error) {
	b.Logger.Title(context.Buildpack)
	result := libcnb.NewBuildResult()

	dr, err := libpak.NewDependencyResolver(context)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("could not create dependency resolver\n%w", err)
	}

	dc, err := libpak.NewDependencyCache(context)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("could not create dependency cache\n%w", err)
	}
	dc.Logger = b.Logger

	cr, err := libpak.NewConfigurationResolver(context.Buildpack, &b.Logger)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("could not create configuration resolver\n%w", err)
	}

	version, manuallySet := cr.Resolve("BP_OPENLIBERTY_VERSION")
	if manuallySet {
		b.Logger.Infof("Choosing user-defined version %s for Open Liberty runtime", version)
	} else {
		b.Logger.Infof("Choosing default version %s for Open Liberty runtime", version)
	}

	profile, manuallySet := cr.Resolve("BP_OPENLIBERTY_PROFILE")
	if manuallySet {
		b.Logger.Infof("Choosing user-defined profile %s for Open Liberty runtime", profile)
	} else {
		b.Logger.Infof("Choosing default profile %s for Open Liberty runtime", profile)
	}

	dep, err := dr.Resolve(fmt.Sprintf("open-liberty-runtime-%s", profile), version)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("could not resolve dependency: %w", err)
	}

	h, be := libpak.NewHelperLayer(context.Buildpack, "linker")
	h.Logger = b.Logger

	result.Layers = append(result.Layers, h)
	result.BOM.Entries = append(result.BOM.Entries, be)

	distro, bomEntry := NewDistribution(dep, dc, context.Application.Path)
	distro.Logger = b.Logger

	result.Layers = append(result.Layers, distro)
	result.BOM.Entries = append(result.BOM.Entries, bomEntry)

	result.Processes = []libcnb.Process{
		{
			Type:      "web",
			Command:   "server",
			Arguments: []string{"run", "defaultServer"},
			Default:   true,
		},
	}

	return result, nil

}
