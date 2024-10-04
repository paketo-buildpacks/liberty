/*
 * Copyright 2018-2022 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/libjvm"
	"github.com/paketo-buildpacks/libpak/sherpa"
)

// IsJvmApplicationPackage will return true if `META-INF/application.xml` or `WEB-INF/` exists, which happens when a
// compiled artifact is supplied.
func IsJvmApplicationPackage(appPath string) (bool, error) {
	if appXMLPresent, err := sherpa.FileExists(filepath.Join(appPath, "META-INF", "application.xml")); err != nil {
		return false, fmt.Errorf("unable to check application.xml\n%w", err)
	} else if appXMLPresent {
		return true, nil
	}

	if webInfPresent, err := sherpa.DirExists(filepath.Join(appPath, "WEB-INF")); err != nil {
		return false, fmt.Errorf("unable to check WEB-INF\n%w", err)
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

func GetApps(path string) ([]string, error) {
	if exists, err := sherpa.DirExists(path); err != nil {
		return []string{}, err
	} else if !exists {
		return []string{}, nil
	}

	// Return the empty list for expanded EAR applications
	if exists, err := sherpa.FileExists(filepath.Join(path, "META-INF", "application.xml")); err != nil {
		return []string{}, err
	} else if exists {
		return []string{}, nil
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return []string{}, err
	}

	var apps []string
	for _, file := range files {
		if name := file.Name(); strings.HasSuffix(name, ".war") || strings.HasSuffix(name, ".ear") {
			apps = append(apps, filepath.Join(path, name))
		}
	}
	return apps, nil
}

// HasCompiledArtifacts checks if the given directory has any web or enterprise archives.
func HasCompiledArtifacts(path string) (bool, error) {
	apps, err := GetApps(path)
	if err != nil {
		return false, err
	}
	return len(apps) > 0, nil
}
