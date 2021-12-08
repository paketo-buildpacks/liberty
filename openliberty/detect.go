package openliberty

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libjvm"
	"github.com/paketo-buildpacks/libpak/bard"
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

func (d Detect) Detect(context libcnb.DetectContext) (libcnb.DetectResult, error) {
	result := libcnb.DetectResult{
		Pass: true,
		Plans: []libcnb.BuildPlan{
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
		},
	}

	mainClassPresent, err := d.checkForMainClassInManifest(context)
	if err != nil {
		return libcnb.DetectResult{}, fmt.Errorf("unable to check manifest\n%w", err)
	}
	if mainClassPresent {
		return libcnb.DetectResult{Pass: false}, nil
	}

	applicationXMLPresent, err := d.checkForApplicationXML(context)
	if err != nil {
		return libcnb.DetectResult{}, fmt.Errorf("unable to check application XML\n%w", err)
	}

	webInfPresent, err := d.checkForWebInf(context)
	if err != nil {
		return libcnb.DetectResult{}, fmt.Errorf("unable to check WEB-INF\n%w", err)
	}

	if webInfPresent || applicationXMLPresent {
		result.Plans[0].Provides = append(result.Plans[0].Provides, libcnb.BuildPlanProvide{Name: PlanEntryJVMApplicationPackage})
		return result, nil
	}

	return result, nil
}

// checkForApplicationXML will return true if `META-INF/application.xml` is present, which happens when precompiled bits are provided
func (d Detect) checkForApplicationXML(context libcnb.DetectContext) (bool, error) {
	_, err := os.Stat(filepath.Join(context.Application.Path, "META-INF", "application.xml"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// checkForWebInf will return true if `WEB-INF/` exists, which happens when precompiled bits are provided
func (d Detect) checkForWebInf(context libcnb.DetectContext) (bool, error) {
	info, err := os.Stat(filepath.Join(context.Application.Path, "WEB-INF"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	return info.IsDir(), nil
}

// checkForMainClassInManifest will return true if Main-Class is present in `META-INF/MANIFEST.MF`
func (d Detect) checkForMainClassInManifest(context libcnb.DetectContext) (bool, error) {
	m, err := libjvm.NewManifest(context.Application.Path)
	if err != nil {
		return false, fmt.Errorf("unable to read manifest\n%w", err)
	}

	_, ok := m.Get("Main-Class")
	return ok, nil
}
