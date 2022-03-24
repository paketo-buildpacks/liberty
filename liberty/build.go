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

package liberty

import (
	"fmt"

	"strings"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/liberty/internal/core"
	"github.com/paketo-buildpacks/liberty/internal/server"
	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
)

const (
	openLibertyInstall          = "ol"
	websphereLibertyInstall     = "wlp"
	noneInstall                 = "none"
	openLibertyStackRuntimeRoot = "/opt/ol"
	webSphereLibertyRuntimeRoot = "/opt/ibm"
	javaAppServerLiberty        = "liberty"
	ifixesRoot                  = "/ifixes"
)

type Build struct {
	Executor effect.Executor
	Logger   bard.Logger
}

func (b Build) Build(context libcnb.BuildContext) (libcnb.BuildResult, error) {
	b.Logger.Title(context.Buildpack)

	result := libcnb.NewBuildResult()

	cr, err := libpak.NewConfigurationResolver(context.Buildpack, nil) // nil so we don't log config table
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create configuration resolver\n%w", err)
	}

	appServer, _ := cr.Resolve("BP_JAVA_APP_SERVER")
	if appServer != "" && appServer != javaAppServerLiberty {
		for _, entry := range context.Plan.Entries {
			result.Unmet = append(result.Unmet, libcnb.UnmetPlanEntry{Name: entry.Name})
		}
		return result, nil
	}

	dr, err := libpak.NewDependencyResolver(context)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create dependency resolver\n%w", err)
	}

	dc, err := libpak.NewDependencyCache(context)
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create dependency cache\n%w", err)
	}
	dc.Logger = b.Logger

	cr, err = libpak.NewConfigurationResolver(context.Buildpack, &b.Logger) // recreate so that config table is logged after the title
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create configuration resolver\n%w", err)
	}

	serverName, _ := cr.Resolve("BP_LIBERTY_SERVER_NAME")
	serverBuildSrc := core.NewServerBuildSource(context.Application.Path, serverName, b.Logger)
	appBuildSrc := core.NewAppBuildSource(context.Application.Path, core.JavaAppServerLiberty, b.Logger)

	buildSources := []core.BuildSource{
		serverBuildSrc,
		appBuildSrc,
	}

	var detectedBuildSrc core.BuildSource

	for _, buildSrc := range buildSources {
		b.Logger.Debugf("Checking build source '%s'", buildSrc.Name())
		ok, err := buildSrc.Detect()
		if err != nil {
			return libcnb.BuildResult{},
				fmt.Errorf("unable to detect build source '%s'\n%w", buildSrc.Name(), err)
		}
		if !ok {
			continue
		}

		validApp, err := buildSrc.ValidateApp()
		if err != nil {
			return libcnb.BuildResult{},
				fmt.Errorf("unable to validate build source '%s'\n%w", buildSrc.Name(), err)
		}
		if validApp {
			detectedBuildSrc = buildSrc
			break
		}
	}

	if detectedBuildSrc == nil {
		for _, entry := range context.Plan.Entries {
			result.Unmet = append(result.Unmet, libcnb.UnmetPlanEntry{Name: entry.Name})
		}
		return result, nil
	}

	if serverName == "" {
		serverName, err = detectedBuildSrc.DefaultServerName()
		if err != nil {
			return libcnb.BuildResult{},
				fmt.Errorf("unable to get default server name for '%s'\n%w", detectedBuildSrc.Name(), err)
		}
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
		features, _ := cr.Resolve("BP_LIBERTY_FEATURES")
		featureList := strings.Fields(features)

		ifixes, err := server.LoadIFixesList(ifixesRoot)
		if err != nil {
			return libcnb.BuildResult{}, fmt.Errorf("unable to load ifixes\n%w", err)
		}

		distro, bomEntry := NewDistribution(dep, dc, serverName, context.Application.Path, featureList, ifixes, b.Executor)
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
