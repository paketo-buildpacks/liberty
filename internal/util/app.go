package util

import (
	"fmt"
	"github.com/paketo-buildpacks/libjvm"
	"path/filepath"
)

// IsJvmApplicationPackage will return true if `META-INF/application.xml` or `WEB-INF/` exists, which happens when a
// compiled artifact is supplied.
func IsJvmApplicationPackage(appPath string) (bool, error) {
	if appXMLPresent, err := FileExists(filepath.Join(appPath, "META-INF", "application.xml")); err != nil {
		return false, fmt.Errorf("unable to check application.xml:\n%w", err)
	} else if appXMLPresent {
		return true, nil
	}

	if webInfPresent, err := DirExists(filepath.Join(appPath, "WEB-INF")); err != nil {
		return false, fmt.Errorf("unable to check WEB-INF:\n%w", err)
	} else if webInfPresent {
		return true, nil
	}

	return false, nil
}

// ManifestHasMainClassDefined will return true if Main-Class is present in `META-INF/MANIFEST.MF`
func ManifestHasMainClassDefined(appPath string) (bool, error) {
	m, err := libjvm.NewManifest(appPath)
	if err != nil {
		return false, fmt.Errorf("unable to read manifest\n%w", err)
	}

	_, ok := m.Get("Main-Class")
	return ok, nil
}
