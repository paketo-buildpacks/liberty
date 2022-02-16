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
	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/open-liberty/internal/server"
	"github.com/paketo-buildpacks/open-liberty/internal/util"
)

const (
	openLibertyInstall      = "ol"
	websphereLibertyInstall = "wlp"
	noneInstall             = "none"

	openLibertyStackRuntimeRoot = "/opt/ol"
	webSphereLibertyRuntimeRoot = "/opt/ibm"
)

type Build struct {
	Logger bard.Logger
}

func (b Build) Build(context libcnb.BuildContext) (libcnb.BuildResult, error) {
	result := libcnb.NewBuildResult()

	if hasApp, err := b.checkJvmApplicationProvided(context); err != nil {
		return libcnb.BuildResult{}, err
	} else if !hasApp {
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

	version, _ := cr.Resolve("BP_OPENLIBERTY_VERSION")
	profile, _ := cr.Resolve("BP_OPENLIBERTY_PROFILE")

	dep, err := dr.Resolve(fmt.Sprintf("open-liberty-runtime-%s", profile), version)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("could not resolve dependency\n%w", err)
	}

	h, be := libpak.NewHelperLayer(context.Buildpack, "linker")
	h.Logger = b.Logger

	result.Layers = append(result.Layers, h)
	result.BOM.Entries = append(result.BOM.Entries, be)

	var externalConfigurationDependency *libpak.BuildpackDependency
	if uri, ok := cr.Resolve("BP_OPENLIBERTY_EXT_CONF_URI"); ok {
		v, _ := cr.Resolve("BP_OPENLIBERTY_EXT_CONF_VERSION")
		s, _ := cr.Resolve("BP_OPENLIBERTY_EXT_CONF_SHA256")

		externalConfigurationDependency = &libpak.BuildpackDependency{
			ID:      "open-liberty-external-configuration",
			Name:    "Open Liberty External Configuration",
			Version: v,
			URI:     uri,
			SHA256:  s,
			Stacks:  []string{context.StackID},
			CPEs:    nil,
			PURL:    "",
		}
	}

	base := NewBase(context.Buildpack.Path, externalConfigurationDependency, cr, dc)
	base.Logger = b.Logger
	result.Layers = append(result.Layers, base)

	installType, _ := cr.Resolve("BP_OPENLIBERTY_INSTALL_TYPE")
	if installType == openLibertyInstall {
		// Provide the OL distribution
		distro, bomEntry := NewDistribution(dep, dc, context.Application.Path)
		distro.Logger = b.Logger

		result.Layers = append(result.Layers, distro)
		result.BOM.Entries = append(result.BOM.Entries, bomEntry)
	}

	result.Processes, err = b.createProcesses(installType)
	if err != nil {
		return libcnb.BuildResult{}, err
	}

	return result, nil
}

func (b Build) createProcesses(installType string) ([]libcnb.Process, error) {
	var processType string
	var command string
	var args []string

	if installType == openLibertyInstall {
		processType = "open-liberty"
		command = "server"
		args = []string{"run", "defaultServer"}
	} else if installType == noneInstall {
		var runtimeRoot string

		// Determine the runtime provided in the stack
		if olExists, err := util.DirExists(openLibertyStackRuntimeRoot); err != nil {
			return []libcnb.Process{}, fmt.Errorf("unable to check Open Liberty stack runtime root exists\n%w", err)
		} else if olExists {
			runtimeRoot = openLibertyStackRuntimeRoot
			processType = "open-liberty-stack"
		} else if wlpExists, err := util.DirExists(webSphereLibertyRuntimeRoot); err != nil {
			return []libcnb.Process{}, fmt.Errorf("unable to check WebSphere Liberty stack runtime root exists\n%w", err)
		} else if wlpExists {
			runtimeRoot = webSphereLibertyRuntimeRoot
			processType = "websphere-liberty-stack"
		} else {
			return []libcnb.Process{}, fmt.Errorf("unable to find server in the stack image")
		}

		b.Logger.Debugf("Using Liberty runtime provided found at %v", runtimeRoot)
		command = "docker-server.sh"
		args = []string{"server", "run", "defaultServer"}
	} else {
		return []libcnb.Process{}, fmt.Errorf("unable to process install type: '%v'", installType)
	}

	b.Logger.Debugf("Using command '%v' and arguments: '%v'", command, args)

	process := libcnb.Process{
		Type:      processType,
		Command:   command,
		Arguments: args,
		Default:   true,
		Direct:    true,
	}

	return []libcnb.Process{process}, nil
}

func (b Build) checkJvmApplicationProvided(context libcnb.BuildContext) (bool, error) {
	isPackagedServer := isPackagedServerPlan(context.Plan.Entries)
	if isPackagedServer {
		return b.validatePackagedServer(context.Application.Path, DefaultServerName)
	}
	return b.validateApplication(context.Application.Path)
}

// validatePackagedServer returns true if a server.xml is found and at least one app is installed.
func (b Build) validatePackagedServer(serverRoot, serverName string) (bool, error) {
	libertyServer := server.LibertyServer{
		InstallRoot: serverRoot,
		ServerName:  serverName,
	}

	if serverConfigFound, err := util.FileExists(libertyServer.GetServerConfigPath()); err != nil {
		return false, fmt.Errorf("unable to read server.xml\n%w", err)
	} else if !serverConfigFound {
		b.Logger.Debug("Server config not found, skipping build")
		return false, nil
	}

	if hasApps, err := libertyServer.HasInstalledApps(); err != nil {
		return false, err
	} else if !hasApps {
		b.Logger.Debug("No apps found in packaged server, skipping build")
		return false, nil
	}

	return true, nil
}

// validateApplication returns true if `Main-Class` is not be defined in the application manifest and either of the
// following files exist: `META-INF/application.xml` or `WEB-INF/`.
func (b Build) validateApplication(appRoot string) (bool, error) {
	// Check contributed app if it is valid
	if isMainClassDefined, err := util.ManifestHasMainClassDefined(appRoot); err != nil {
		return false, err
	} else if isMainClassDefined {
		b.Logger.Debug("`Main-Class` found in `META-INF/MANIFEST.MF`, skipping build")
		return false, nil
	}

	if isJvmAppPackage, err := util.IsJvmApplicationPackage(appRoot); err != nil {
		return false, err
	} else if !isJvmAppPackage {
		b.Logger.Debug("No `WEB-INF/` or `META-INF/application.xml` found, skipping build")
		return false, nil
	}

	return true, nil
}

func isPackagedServerPlan(plans []libcnb.BuildpackPlanEntry) bool {
	var value bool
	for _, entry := range plans {
		if entry.Name == PlanEntryOpenLiberty {
			if packagedServerValue, found := entry.Metadata["packaged-server"]; found {
				val, ok := packagedServerValue.(bool)
				value = ok && val
			}
			break
		}
	}
	return value
}
