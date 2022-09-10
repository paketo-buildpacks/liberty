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
	"github.com/heroku/color"
	"github.com/paketo-buildpacks/libpak/bindings"
	sherpa "github.com/paketo-buildpacks/libpak/sherpa"
	"strings"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/liberty/internal/core"
	"github.com/paketo-buildpacks/liberty/internal/server"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/sbom"
)

const (
	openLibertyInstall          = "ol"
	websphereLibertyInstall     = "wlp"
	noneInstall                 = "none"
	openLibertyStackRuntimeRoot = "/opt/ol"
	webSphereLibertyRuntimeRoot = "/opt/ibm"
	javaAppServerLiberty        = "liberty"
	ifixesRoot                  = "/ifixes"
	featuresRoot                = "/features"
)

type Build struct {
	Executor    effect.Executor
	Logger      bard.Logger
	SBOMScanner sbom.SBOMScanner
}

func (b Build) Build(context libcnb.BuildContext) (libcnb.BuildResult, error) {
	result := libcnb.NewBuildResult()

	pr := libpak.PlanEntryResolver{Plan: context.Plan}
	if _, found, err := pr.Resolve(PlanEntryJavaAppServer); err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to resolve plan entry\n%w", err)
	} else if !found {
		return result, nil
	}

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
			b.Logger.Debugf("Detect failed for '%s' -- skipping", buildSrc.Name())
			continue
		}

		b.Logger.Debugf("Validating app for build source '%s'", buildSrc.Name())
		validApp, err := buildSrc.ValidateApp()
		if err != nil {
			return libcnb.BuildResult{},
				fmt.Errorf("unable to validate build source '%s'\n%w", buildSrc.Name(), err)
		}
		if validApp {
			detectedBuildSrc = buildSrc
			break
		} else {
			b.Logger.Debugf("Validation failed for '%s' -- skipping", buildSrc.Name())
		}
	}

	if detectedBuildSrc == nil {
		for _, entry := range context.Plan.Entries {
			result.Unmet = append(result.Unmet, libcnb.UnmetPlanEntry{Name: entry.Name})
		}
		return result, nil
	}

	b.Logger.Title(context.Buildpack)

	cr, err = libpak.NewConfigurationResolver(context.Buildpack, &b.Logger) // recreate so that config table is logged after the title
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to create configuration resolver\n%w", err)
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

	h := libpak.NewHelperLayerContributor(context.Buildpack, "linker")
	h.Logger = b.Logger
	result.Layers = append(result.Layers, h)

	if serverName == "" {
		serverName, err = detectedBuildSrc.DefaultServerName()
		if err != nil {
			return libcnb.BuildResult{},
				fmt.Errorf("unable to get default server name for '%s'\n%w", detectedBuildSrc.Name(), err)
		}
	}

	if b.SBOMScanner == nil {
		b.SBOMScanner = sbom.NewSyftCLISBOMScanner(context.Layers, b.Executor, b.Logger)
	}

	installType, _ := cr.Resolve("BP_LIBERTY_INSTALL_TYPE")
	profile, _ := cr.Resolve("BP_LIBERTY_PROFILE")
	if profile == "" {
		if installType == openLibertyInstall {
			b.Logger.Info(color.YellowString("Warning: The default profile for Open Liberty will change from 'full' to 'kernel' after 2022-11-01. To continue using the full profile, build with the argument '--env BP_LIBERTY_PROFILE=full'"))
			profile = "full"
		} else if installType == websphereLibertyInstall {
			profile = "kernel"
		}
	}

	isValidProfile := true
	if installType == openLibertyInstall {
		isValidProfile = server.IsValidOpenLibertyProfile(profile)
	} else if installType == websphereLibertyInstall {
		isValidProfile = server.IsValidWebSphereLibertyProfile(profile)
	}
	if !isValidProfile {
		return libcnb.BuildResult{}, fmt.Errorf("invalid profile '%s' for BP_INSTALL_TYPE '%s'", profile, installType)
	}

	version, _ := cr.Resolve("BP_LIBERTY_VERSION")
	features, _ := cr.Resolve("BP_LIBERTY_FEATURES")
	featureList := strings.Fields(features)
	appPath, err := detectedBuildSrc.AppPath()
	if err != nil {
		return libcnb.BuildResult{}, err
	}
	featureList, err = server.GetFeatureList(profile, appPath, featureList)
	if err != nil {
		return libcnb.BuildResult{}, err
	}
	userFeatureDescriptor, err := ReadFeatureDescriptor(featuresRoot, b.Logger)
	if err != nil {
		return libcnb.BuildResult{}, err
	}
	binding, _, err := bindings.ResolveOne(context.Platform.Bindings, bindings.OfType("liberty"))
	if err != nil {
		return libcnb.BuildResult{}, fmt.Errorf("unable to resolve liberty bindings\n%w", err)
	}
	base := NewBase(
		context.Application.Path,
		context.Buildpack.Path,
		serverName,
		featureList,
		userFeatureDescriptor,
		binding,
		b.Logger,
	)
	result.Layers = append(result.Layers, base)

	if installType == openLibertyInstall || installType == websphereLibertyInstall {
		if err := b.buildDistributionRuntime(profile, version, installType, serverName, context.Application.Path, featureList, detectedBuildSrc, dr, dc, &result); err != nil {
			return libcnb.BuildResult{}, err
		}
	} else if installType == noneInstall {
		if err := b.buildStackRuntime(serverName, &result); err != nil {
			return libcnb.BuildResult{}, err
		}
	} else {
		return libcnb.BuildResult{}, fmt.Errorf("unable to process install type: '%s'", installType)
	}

	return result, nil
}

func (b Build) buildDistributionRuntime(
	profile string,
	version string,
	installType string,
	serverName string,
	appPath string,
	features []string,
	buildSrc core.BuildSource,
	dependencyResolver libpak.DependencyResolver,
	cache libpak.DependencyCache,
	result *libcnb.BuildResult) error {

	var distType string

	if installType == openLibertyInstall {
		distType = "open-liberty-runtime"
	} else if installType == websphereLibertyInstall {
		distType = "websphere-liberty-runtime"
	}

	dep, err := dependencyResolver.Resolve(fmt.Sprintf("%s-%s", distType, profile), version)
	if err != nil {
		return fmt.Errorf("unable to resolve dependency\n%w", err)
	}

	// Provide the Liberty distribution
	iFixes, err := server.LoadIFixesList(ifixesRoot)
	if err != nil {
		return fmt.Errorf("unable to load iFixes\n%w", err)
	}

	distro := NewDistribution(dep, cache, serverName, appPath, features, iFixes, b.Executor)
	distro.Logger = b.Logger

	result.Layers = append(result.Layers, distro)
	result.Processes = []libcnb.Process{
		{
			Type:      distType,
			Command:   "server",
			Arguments: []string{"run", serverName},
			Default:   true,
			Direct:    true,
		},
	}

	scanPath, err := buildSrc.AppPath()
	if err != nil {
		return fmt.Errorf("unable to find scan path\n%s", err)
	}

	if err := b.SBOMScanner.ScanLaunch(scanPath, libcnb.SyftJSON, libcnb.CycloneDXJSON); err != nil {
		return fmt.Errorf("unable to create Launch SBoM \n%w", err)
	}

	return nil
}

func (b Build) buildStackRuntime(serverName string, result *libcnb.BuildResult) error {
	process, err := createStackRuntimeProcess(serverName)
	if err != nil {
		return err
	}
	result.Processes = []libcnb.Process{process}
	return nil
}

func createStackRuntimeProcess(serverName string) (libcnb.Process, error) {
	olExists, err := sherpa.DirExists(openLibertyStackRuntimeRoot)
	if err != nil {
		return libcnb.Process{}, fmt.Errorf("unable to check Open Liberty stack runtime root exists\n%w", err)
	}
	if olExists {
		return libcnb.Process{
			Type:      "open-liberty-stack",
			Command:   "bootstrap.sh",
			Arguments: []string{"server", "run", serverName},
			Default:   true,
			Direct:    true,
		}, nil
	}

	wlpExists, err := sherpa.DirExists(webSphereLibertyRuntimeRoot)
	if err != nil {
		return libcnb.Process{}, fmt.Errorf("unable to WebSphere Open Liberty stack runtime root exists\n%w", err)
	}
	if wlpExists {
		return libcnb.Process{
			Type:      "websphere-liberty-stack",
			Command:   "bootstrap.sh",
			Arguments: []string{"server", "run", serverName},
			Default:   true,
			Direct:    true,
		}, nil
	}

	return libcnb.Process{}, fmt.Errorf("unable to find server in the stack image")
}
