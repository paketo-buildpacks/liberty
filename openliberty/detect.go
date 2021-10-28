package openliberty

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libjvm"
)

const (
	PlanEntryOpenLiberty           = "open-liberty"
	PlanEntryJDK                   = "jdk"
	PlanEntryJRE                   = "jre"
	PlanEntryJVMApplicationPackage = "jvm-application-package"
	PlanEntryJVMApplication        = "jvm-application"
)

type Detect struct{}

type detector func(libcnb.DetectContext) (bool, error)

func (d Detect) Detect(context libcnb.DetectContext) (libcnb.DetectResult, error) {
	result := libcnb.DetectResult{}
	var err error

	fullDetector := d.and(
		d.or(
			d.checkWebInf,
			d.checkRootServerXML,
			d.checkNestedServerXML,
		),
		d.checkManifest,
	)

	if result.Pass, err = fullDetector(context); err != nil {
		return libcnb.DetectResult{}, err
	}

	if result.Pass {
		result.Plans = []libcnb.BuildPlan{
			{
				Provides: []libcnb.BuildPlanProvide{
					{Name: PlanEntryOpenLiberty},
					{Name: PlanEntryJVMApplication},
				},

				Requires: []libcnb.BuildPlanRequire{
					{Name: PlanEntryJDK, Metadata: map[string]interface{}{"build": true}},
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
	serverXML := filepath.Join(context.Application.Path, "server.xml")
	if _, err := os.Stat(serverXML); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (d Detect) checkNestedServerXML(context libcnb.DetectContext) (bool, error) {
	glob := filepath.Join(context.Application.Path, "wlp", "usr", "servers", "*", "server.xml")
	matches, err := filepath.Glob(glob)

	return len(matches) > 0, err
}

func (d Detect) checkWebInf(context libcnb.DetectContext) (bool, error) {
	path := filepath.Join(context.Application.Path, "WEB-INF")
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
