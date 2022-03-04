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
	"io/ioutil"
	"path/filepath"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/liberty/internal/server"
	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
)

const (
	PlanEntryLiberty               = "liberty"
	PlanEntryJRE                   = "jre"
	PlanEntryJVMApplicationPackage = "jvm-application-package"
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
	usrPath, isPackagedServer, err := d.getPackagedServerUsrPath(context.Application.Path)
	if err != nil {
		return libcnb.DetectResult{}, fmt.Errorf("unable to detect packaged server path\n%w", err)
	}
	if !isPackagedServer {
		d.Logger.Debug("Detected application")
		return d.detectApplication(context.Application.Path, serverName)
	}

	d.Logger.Debug("Detected packaged server")
	serverName, err = d.detectPackagedServerName(filepath.Join(usrPath, "servers"), serverName)
	if err != nil {
		return libcnb.DetectResult{}, fmt.Errorf("unable to detect packaged server name\n%w", err)
	}
	return d.detectPackagedServer(usrPath, serverName)
}

// detectApplication will handle detection of applications. It will pass detection iff `Main-Class` is not defined in
// the manifest. If a compiled artifact was pushed, detectApplication will mark the `jvm-application-package`
// requirement as being met.
func (d Detect) detectApplication(appPath, serverName string) (libcnb.DetectResult, error) {
	mainClassDefined, err := util.ManifestHasMainClassDefined(appPath)
	if err != nil {
		return libcnb.DetectResult{}, fmt.Errorf("unable to check manifest\n%w", err)
	}
	if mainClassDefined {
		return libcnb.DetectResult{Pass: false}, nil
	}

	// When a compiled artifact is pushed, mark that a JVM application package has been provided so that the build
	// plan requirement is satisfied.
	isJvmAppPackage, err := util.IsJvmApplicationPackage(appPath)
	if err != nil {
		return libcnb.DetectResult{}, err
	}

	if serverName == "" {
		serverName = "defaultServer"
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
					{Name: PlanEntryJVMApplicationPackage},
					{Name: PlanEntryLiberty, Metadata: map[string]interface{}{
						"server-name": serverName,
					}},
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
					{Name: PlanEntryJVMApplicationPackage},
					{Name: PlanEntryLiberty, Metadata: map[string]interface{}{
						"packaged-server-usr-path": serverUserPath,
						"server-name":              serverName,
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

func (d Detect) getPackagedServerUsrPath(appPath string) (string, bool, error) {
	dirs := []string{
		filepath.Join("wlp", "usr"),
		"usr",
	}
	for _, dir := range dirs {
		usrPath := filepath.Join(appPath, dir)
		exists, err := util.DirExists(filepath.Join(usrPath, "servers"))
		if err != nil {
			return "", false, err
		}
		if exists {
			return usrPath, true, nil
		}
	}
	return "", false, nil
}

func (d Detect) detectPackagedServerName(serversPath, serverName string) (string, error) {
	if exists, err := util.DirExists(serversPath); err != nil || !exists {
		return "", err
	}

	// If a serverName was not provided via BP_LIBERTY_SERVER_NAME, try to detect server name
	if serverName == "" {
		servers, err := ioutil.ReadDir(serversPath)
		if err != nil {
			return "", err
		}
		if numServers := len(servers); numServers == 0 {
			return "", fmt.Errorf("unable to determine which server to use -- no servers detected")
		} else if numServers > 1 {
			return "", fmt.Errorf("unable to determine which server to use -- more than one server detected; specify the desired server using BP_LIBERTY_SERVER_NAME\ndetected servers: %v", servers)
		}
		serverName = servers[0].Name()
	}

	configPath := filepath.Join(serversPath, serverName, "server.xml")
	exists, err := util.FileExists(configPath)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("server.xml not found for server '%s' at '%s'", serverName, configPath)
	}
	return serverName, nil
}
