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
	"github.com/paketo-buildpacks/libjvm"
	"path/filepath"
)

// IsJvmApplicationPackage will return true if `META-INF/application.xml` or `WEB-INF/` exists, which happens when a
// compiled artifact is supplied.
func IsJvmApplicationPackage(appPath string) (bool, error) {
	if appXMLPresent, err := FileExists(filepath.Join(appPath, "META-INF", "application.xml")); err != nil {
		return false, fmt.Errorf("unable to check application.xml\n%w", err)
	} else if appXMLPresent {
		return true, nil
	}

	if webInfPresent, err := DirExists(filepath.Join(appPath, "WEB-INF")); err != nil {
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
