/*
 * Copyright 2018-2020 the original author or authors.
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

package openliberty

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libjvm"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
)

type Build struct {
	Logger bard.Logger
}

func (b Build) Build(context libcnb.BuildContext) (libcnb.BuildResult, error) {
	result := libcnb.NewBuildResult()

	m, err := libjvm.NewManifest(context.Application.Path)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to read manifest\n%w", err)
	}

	if _, ok := m.Get("Main-Class"); ok {
		b.Logger.Debug("`Main-Class` found in `META-INF/MANIFEST.MF`, skipping build")
		for _, entry := range context.Plan.Entries {
			result.Unmet = append(result.Unmet, libcnb.UnmetPlanEntry{Name: entry.Name})
		}
		return result, nil
	}

	var webInfMissing bool
	webInfPath := filepath.Join(context.Application.Path, "WEB-INF")
	_, err = os.Stat(webInfPath)
	if err != nil && !os.IsNotExist(err) {
		return libcnb.BuildResult{}, fmt.Errorf("unable to stat file %s\n%w", webInfPath, err)
	} else if os.IsNotExist(err) {
		webInfMissing = true
	}

	var appXMLMissing bool
	appXMLPath := filepath.Join(context.Application.Path, "META-INF", "application.xml")
	_, err = os.Stat(appXMLPath)
	if err != nil && !os.IsNotExist(err) {
		return libcnb.BuildResult{}, fmt.Errorf("unable to stat file %s\n%w", appXMLPath, err)
	} else if os.IsNotExist(err) {
		appXMLMissing = true
	}

	if webInfMissing && appXMLMissing {
		b.Logger.Debug("No `WEB-INF/` or `META-INF/application.xml` found, skipping build")
		for _, entry := range context.Plan.Entries {
			result.Unmet = append(result.Unmet, libcnb.UnmetPlanEntry{Name: entry.Name})
		}
		return result, nil
	}

	b.Logger.Title(context.Buildpack)

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
