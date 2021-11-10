package openliberty

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libjvm"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/bindings"
)

const (
	PlanEntryOpenLiberty           = "open-liberty"
	PlanEntryJDK                   = "jdk"
	PlanEntryJRE                   = "jre"
	PlanEntryJVMApplicationPackage = "jvm-application-package"
	PlanEntryJVMApplication        = "jvm-application"
)

type Detect struct {
	Logger bard.Logger
}

type detector func(libcnb.DetectContext) (bool, error)

func (d Detect) Detect(context libcnb.DetectContext) (libcnb.DetectResult, error) {
	result := libcnb.DetectResult{}
	var err error

	fullDetector := d.and(
		d.checkManifest,
		d.or(
			d.checkRootServerXML,
			d.checkWebInf,
		),
	)

	if result.Pass, err = fullDetector(context); err != nil {
		return libcnb.DetectResult{}, err
	}

	if result.Pass {
		result.Plans = []libcnb.BuildPlan{
			{
				Provides: []libcnb.BuildPlanProvide{
					{Name: PlanEntryOpenLiberty},
				},

				Requires: []libcnb.BuildPlanRequire{
					{Name: PlanEntryJRE, Metadata: map[string]interface{}{"launch": true}},
					{Name: PlanEntryJVMApplicationPackage},
					{Name: PlanEntryOpenLiberty},
				},
			},
		}
	}

	return result, nil
}

func (d Detect) or(detectors ...detector) detector {
	return func(context libcnb.DetectContext) (bool, error) {
		err := errors.New("")
		var passed bool

		for _, detect := range detectors {
			var tmpErr error
			if passed, tmpErr = detect(context); passed {
				return true, nil
			}

			if tmpErr != nil {
				err = fmt.Errorf("%v\n%w", err, tmpErr)
			}
		}

		if len(err.Error()) > 0 {
			return false, err
		}

		return false, nil
	}
}

func (d Detect) and(detectors ...detector) detector {
	return func(context libcnb.DetectContext) (bool, error) {
		for _, detect := range detectors {
			if passed, err := detect(context); !passed {
				return false, err
			}
		}

		return true, nil
	}
}

func (d Detect) checkRootServerXML(context libcnb.DetectContext) (bool, error) {
	b, ok, err := bindings.ResolveOne(context.Platform.Bindings, bindings.OfType("open-liberty"))
	if err != nil {
		return false, fmt.Errorf("error resolving bindings: %w", err)
	}

	if ok {
		_, ok = b.SecretFilePath("server.xml")
	}

	return ok, nil
}

func (d Detect) checkWebInf(context libcnb.DetectContext) (bool, error) {
	cr, err := libpak.NewConfigurationResolver(context.Buildpack, &d.Logger)
	if err != nil {
		return false, fmt.Errorf("could not create configuration resolver\n%w", err)
	}

	srcPath, _ := cr.Resolve("BP_OPENLIBERTY_WEBINF_PATH")

	path := filepath.Join(context.Application.Path, srcPath, "WEB-INF")
	path, err = filepath.Abs(path)
	if err != nil {
		return false, fmt.Errorf("could not determine absolute path to WEB-INF directory: %w", err)
	}

	// make sure we don't try to escape the app path
	if !strings.HasPrefix(path, context.Application.Path) {
		return false, fmt.Errorf("setting BP_OPENLIBERTY_WEBINF_PATH must result in a path below %s. Requested path: %s", context.Application.Path, path)
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	return info.IsDir(), nil
}

func (d Detect) checkManifest(context libcnb.DetectContext) (bool, error) {
	m, err := libjvm.NewManifest(context.Application.Path)
	if err != nil {
		return false, fmt.Errorf("unable to read manifest\n%w", err)
	}

	if _, ok := m.Get("Main-Class"); ok {
		return false, nil
	}

	return true, nil
}
