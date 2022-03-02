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
	"path/filepath"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/liberty/internal/server"
	"github.com/paketo-buildpacks/liberty/internal/util"
)

const (
	PlanEntryLiberty           	   = "liberty"
	PlanEntryJRE                   = "jre"
	PlanEntryJVMApplicationPackage = "jvm-application-package"
	PlanEntryJavaAppServer		   = "java-app-server"
)

type Detect struct {
	Logger bard.Logger
}

func (d Detect) Detect(context libcnb.DetectContext) (libcnb.DetectResult, error) {
	cr, err := libpak.NewConfigurationResolver(context.Buildpack, nil)
	if err != nil {
		return libcnb.DetectResult{}, fmt.Errorf("unable to create configuration resolver\n%w", err)
	}

	serverName, _ := cr.Resolve("BP_LIBERTY_SERVER_NAME")
	packagedServerDirs := []string{
		filepath.Join("wlp", "usr"),
		"usr",
	}

	for _, dir := range packagedServerDirs {
		serverUserPath := filepath.Join(context.Application.Path, dir)
		isPackagedServer, err := util.FileExists(filepath.Join(serverUserPath, "servers", serverName, "server.xml"))
		if err != nil {
			return libcnb.DetectResult{}, fmt.Errorf("unable to read packaged server.xml\n%w", err)
		}
		if isPackagedServer {
			d.Logger.Debug("Detected packaged server")
			return d.detectPackagedServer(serverUserPath, serverName)
		}
	}

	d.Logger.Debug("Detected application")
	return d.detectApplication(context.Application.Path)
}

// detectApplication will handle detection of applications. It will pass detection iff `Main-Class` is not defined in
// the manifest. If a compiled artifact was pushed, detectApplication will mark the `jvm-application-package`
// requirement as being met.
func (d Detect) detectApplication(appPath string) (libcnb.DetectResult, error) {
	if mainClassDefined, err := util.ManifestHasMainClassDefined(appPath); err != nil {
		return libcnb.DetectResult{}, fmt.Errorf("unable to check manifest\n%w", err)
	} else if mainClassDefined {
		return libcnb.DetectResult{Pass: false}, nil
	}

	// When a compiled artifact is pushed, mark that a JVM application package has been provided so that the build
	// plan requirement is satisfied.
	isJvmAppPackage, err := util.IsJvmApplicationPackage(appPath)
	if err != nil {
		return libcnb.DetectResult{}, err
	}

	result := libcnb.DetectResult{
		Pass: true,
		Plans: []libcnb.BuildPlan{
			{
				Provides: []libcnb.BuildPlanProvide{
					{Name: PlanEntryLiberty},
					{Name: PlanEntryJavaAppServer},
				},

				Requires: []libcnb.BuildPlanRequire{
					{Name: PlanEntryJRE, Metadata: map[string]interface{}{
						"launch": true,
						"build":  true,
						"cache":  true},
					},
					{Name: PlanEntryJavaAppServer},
					{Name: PlanEntryJVMApplicationPackage},
					{Name: PlanEntryLiberty},
				},
			},
		},
	}

	if isJvmAppPackage {
		result.Plans[0].Provides = append(result.Plans[0].Provides, libcnb.BuildPlanProvide{
			Name: PlanEntryJVMApplicationPackage,
		})
	} else {
		d.Logger.Debug("Not a JVM application package")
	}

	return result, nil
}

// detectPackagedServer handles detection of a packaged Liberty server.
func (d Detect) detectPackagedServer(serverUserPath, serverName string) (libcnb.DetectResult, error) {
	libertyServer := server.LibertyServer{
		ServerUserPath: serverUserPath,
		ServerName:     serverName,
	}
	hasApps, err := libertyServer.HasInstalledApps()
	if err != nil {
		return libcnb.DetectResult{}, fmt.Errorf("unable to check if packaged server has apps\n%w", err)
	}

	result := libcnb.DetectResult{
		Pass: true,
		Plans: []libcnb.BuildPlan{
			{
				Provides: []libcnb.BuildPlanProvide{
					{Name: PlanEntryLiberty},
				},

				Requires: []libcnb.BuildPlanRequire{
					{Name: PlanEntryJRE, Metadata: map[string]interface{}{
						"launch": true,
						"build":  true,
						"cache":  true},
					},
					{Name: PlanEntryJavaAppServer},
					{Name: PlanEntryJVMApplicationPackage},
					{Name: PlanEntryLiberty, Metadata: map[string]interface{}{
						"packaged-server":          true,
						"packaged-server-usr-path": serverUserPath,
					}},
				},
			},
		},
	}

	if hasApps {
		result.Plans[0].Provides = append(result.Plans[0].Provides, libcnb.BuildPlanProvide{
			Name: PlanEntryJVMApplicationPackage,
		})
	} else {
		d.Logger.Debug("No applications detected in server bundle")
	}

	return result, nil
}
