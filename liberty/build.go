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

package liberty

import (
	"fmt"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/liberty/internal/server"
	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/paketo-buildpacks/libpak/sherpa"
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
	b.Logger.Title(context.Buildpack)
	
	appServer := sherpa.GetEnvWithDefault("BP_JAVA_APP_SERVER", "")
	if appServer != "" && appServer != "liberty" {
		return libcnb.BuildResult{}, nil
	}	
	
	result := libcnb.NewBuildResult()

	dr, err := libpak.NewDependencyResolver(context)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create dependency resolver\n%w", err)
	}

	dc, err := libpak.NewDependencyCache(context)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create dependency cache\n%w", err)
	}
	dc.Logger = b.Logger

	cr, err := libpak.NewConfigurationResolver(context.Buildpack, &b.Logger)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create configuration resolver\n%w", err)
	}

	serverName, _ := cr.Resolve("BP_LIBERTY_SERVER_NAME")

	if hasApp, err := b.checkJvmApplicationProvided(context, serverName); err != nil {
		return libcnb.BuildResult{}, err
	} else if !hasApp {
		for _, entry := range context.Plan.Entries {
			result.Unmet = append(result.Unmet, libcnb.UnmetPlanEntry{Name: entry.Name})
		}
		return result, nil
	}

	version, _ := cr.Resolve("BP_LIBERTY_VERSION")
	profile, _ := cr.Resolve("BP_LIBERTY_PROFILE")

	dep, err := dr.Resolve(fmt.Sprintf("open-liberty-runtime-%s", profile), version)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to resolve dependency\n%w", err)
	}

	h, be := libpak.NewHelperLayer(context.Buildpack, "linker")
	h.Logger = b.Logger

	result.Layers = append(result.Layers, h)
	result.BOM.Entries = append(result.BOM.Entries, be)

	var externalConfigurationDependency *libpak.BuildpackDependency
	if uri, ok := cr.Resolve("BP_LIBERTY_EXT_CONF_URI"); ok {
		v, _ := cr.Resolve("BP_LIBERTY_EXT_CONF_VERSION")
		s, _ := cr.Resolve("BP_LIBERTY_EXT_CONF_SHA256")

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

	base := NewBase(context.Buildpack.Path, serverName, externalConfigurationDependency, cr, dc)
	base.Logger = b.Logger
	result.Layers = append(result.Layers, base)

	installType, _ := cr.Resolve("BP_LIBERTY_INSTALL_TYPE")
	if installType == openLibertyInstall {
		// Provide the OL distribution
		distro, bomEntry := NewDistribution(dep, dc, serverName, context.Application.Path)
		distro.Logger = b.Logger

		result.Layers = append(result.Layers, distro)
		result.BOM.Entries = append(result.BOM.Entries, bomEntry)

		process, err := createOpenLibertyRuntimeProcess(serverName)
		if err != nil {
			return libcnb.BuildResult{}, err
		}
		result.Processes = []libcnb.Process{process}
	} else if installType == noneInstall {
		process, err := createStackRuntimeProcess(serverName)
		if err != nil {
			return libcnb.BuildResult{}, err
		}
		result.Processes = []libcnb.Process{process}
	} else {
		return libcnb.BuildResult{}, fmt.Errorf("unable to process install type: '%s'", installType)
	}

	return result, nil
}

func createOpenLibertyRuntimeProcess(serverName string) (libcnb.Process, error) {
	return libcnb.Process{
		Type:      "open-liberty",
		Command:   "server",
		Arguments: []string{"run", serverName},
		Default:   true,
		Direct:    true,
	}, nil
}

func createStackRuntimeProcess(serverName string) (libcnb.Process, error) {
	if olExists, err := util.DirExists(openLibertyStackRuntimeRoot); err != nil {
		return libcnb.Process{}, fmt.Errorf("unable to check Open Liberty stack runtime root exists\n%w", err)
	} else if olExists {
		return libcnb.Process{
			Type:      "open-liberty-stack",
			Command:   "docker-server.sh",
			Arguments: []string{"server", "run", serverName},
			Default:   true,
			Direct:    true,
		}, nil
	}

	if wlpExists, err := util.DirExists(webSphereLibertyRuntimeRoot); err != nil {
		return libcnb.Process{}, fmt.Errorf("unable to WebSphere Open Liberty stack runtime root exists\n%w", err)
	} else if wlpExists {
		return libcnb.Process{
			Type:      "websphere-liberty-stack",
			Command:   "docker-server.sh",
			Arguments: []string{"server", "run", serverName},
			Default:   true,
			Direct:    true,
		}, nil
	}

	return libcnb.Process{}, fmt.Errorf("unable to find server in the stack image")
}

func (b Build) checkJvmApplicationProvided(context libcnb.BuildContext, serverName string) (bool, error) {
	isPackagedServer := isPackagedServerPlan(context.Plan.Entries)

	if isPackagedServer {
		serverUserPath, err := getPackagedServerUserPath(context.Plan.Entries)
		if err != nil {
			return false, err
		}
		return b.validatePackagedServer(serverUserPath, serverName)
	}
	return b.validateApplication(context.Application.Path)
}

// validatePackagedServer returns true if a server.xml is found and at least one app is installed.
func (b Build) validatePackagedServer(userPath, serverName string) (bool, error) {
	libertyServer := server.LibertyServer{
		ServerUserPath: userPath,
		ServerName:     serverName,
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
		if entry.Name == PlanEntryLiberty {
			if packagedServerValue, found := entry.Metadata["packaged-server"]; found {
				val, ok := packagedServerValue.(bool)
				value = ok && val
			}
			break
		}
	}
	return value
}

func getPackagedServerUserPath(plans []libcnb.BuildpackPlanEntry) (string, error) {
	for _, entry := range plans {
		if entry.Name == PlanEntryLiberty {
			if userPath, found := entry.Metadata["packaged-server-usr-path"]; found {
				val, ok := userPath.(string)
				if !ok {
					return "", fmt.Errorf("unable to parse packaged-server-usr-path: '%v'", userPath)
				}
				return val, nil
			}
			break
		}
	}
	return "", fmt.Errorf("unable to find packaged-server-usr-path")
}
