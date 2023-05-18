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

package core

import (
	"fmt"
	"github.com/paketo-buildpacks/libpak/sherpa"
	"path/filepath"

	"github.com/paketo-buildpacks/liberty/internal/server"
	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/paketo-buildpacks/libpak/bard"
)

const (
	JavaAppServerLiberty  = "liberty"
	AppBuildSourceName    = "app-build-src"
	ServerBuildSourceName = "svr-build-src"
)

// BuildSource represents different build sources that the Liberty buildpack supports
type BuildSource interface {
	// Name is the name of the build source
	Name() string

	// Detect determines if this build source is the one that should be used
	Detect() (bool, error)

	// DefaultServerName returns the default server name that should be used for the build source
	DefaultServerName() (string, error)

	// ValidateApp returns true if the `jvm-application-package` dependency is provided
	ValidateApp() (bool, error)

	// AppPath returns the list of paths in which the build source contains applications
	AppPath() (string, error)
}

// AppBuildSource is the default build source type for the Liberty buildpack
type AppBuildSource struct {
	RequestedAppServer string
	Root               string
	Logger             bard.Logger
}

// NewAppBuildSource returns the default BuildSource
func NewAppBuildSource(appPath string, requestedAppServer string, logger bard.Logger) AppBuildSource {
	return AppBuildSource{
		RequestedAppServer: requestedAppServer,
		Root:               appPath,
		Logger:             logger,
	}
}

func (a AppBuildSource) Name() string {
	return AppBuildSourceName
}

// Detect checks to make sure Main-Class is not defined in `META-INF/MANIFEST.MF`
func (a AppBuildSource) Detect() (bool, error) {
	// If user requests an app server, and it is not `liberty` then skip
	if a.RequestedAppServer != "" && a.RequestedAppServer != JavaAppServerLiberty {
		a.Logger.Infof("SKIPPED: failed to match requested app server of [%s], buildpack supports [%s]", a.RequestedAppServer, JavaAppServerLiberty)
		return false, nil
	}

	// Check contributed app if it is valid
	isMainClassDefined, err := util.ManifestHasMainClassDefined(a.Root)
	if err != nil {
		return false, err
	}
	if isMainClassDefined {
		a.Logger.Info("SKIPPED: `Main-Class` found in `META-INF/MANIFEST.MF`, skipping build")
		return false, nil
	}

	return true, nil
}

func (a AppBuildSource) DefaultServerName() (string, error) {
	return "defaultServer", nil
}

func (a AppBuildSource) ValidateApp() (bool, error) {
	isAppPackage, err := util.IsJvmApplicationPackage(a.Root)
	if err != nil {
		return false, err
	}
	if isAppPackage {
		return true, nil
	} else {
		a.Logger.Debug("No `WEB-INF/` or `META-INF/application.xml` found")
	}

	// Check if there is a compiled artifact provided
	hasApps, err := util.HasCompiledArtifacts(a.Root)
	if err != nil {
		return false, nil
	}
	if !hasApps {
		a.Logger.Debug("No compiled artifacts found")
	}
	return hasApps, nil
}

func (a AppBuildSource) AppPath() (string, error) {
	return a.Root, nil
}

// ServerBuildSource is used when building a packaged server or Liberty server directory
type ServerBuildSource struct {
	// InstallRoot is the Liberty installation directory where `wlp` or `usr` is found
	InstallRoot string
	// ServerName is the serve instance that built
	ServerName string
	Logger     bard.Logger
}

func NewServerBuildSource(installRoot string, serverName string, logger bard.Logger) ServerBuildSource {
	return ServerBuildSource{
		InstallRoot: installRoot,
		ServerName:  serverName,
		Logger:      logger,
	}
}

func (s ServerBuildSource) Name() string {
	return ServerBuildSourceName
}

func (s ServerBuildSource) Detect() (bool, error) {
	serverPath, err := s.ServerPath()
	if err != nil {
		return false, fmt.Errorf("unable to determine server path\n%w", err)
	}

	if serverPath == "" {
		return false, nil
	}

	return sherpa.FileExists(filepath.Join(serverPath, "server.xml"))
}

func (s ServerBuildSource) DefaultServerName() (string, error) {
	userPath, err := s.UserPath()
	if err != nil {
		return "", fmt.Errorf("unable to determine default server name\n%w", err)
	}
	if userPath == "" {
		return "", fmt.Errorf("unable to determine default server name\n%w", err)
	}

	servers, err := server.GetServerList(userPath)
	if err != nil {
		return "", err
	}
	if numServers := len(servers); numServers == 0 {
		return "", fmt.Errorf("unable to determine which server to use -- no servers detected")
	} else if numServers > 1 {
		return "", fmt.Errorf("unable to determine which server to use -- more than one server detected; specify the desired server using BP_LIBERTY_SERVER_NAME\ndetected servers: %v", servers)
	}
	return servers[0], nil
}

func (s ServerBuildSource) ValidateApp() (bool, error) {
	serverPath, err := s.ServerPath()
	if err != nil {
		return false, fmt.Errorf("unable to validate packaged server app\n%w", err)
	}
	if serverPath == "" {
		return false, nil
	}

	return server.HasInstalledApps(serverPath)
}

func (s ServerBuildSource) AppPath() (string, error) {
	serverPath, err := s.ServerPath()
	if err != nil {
		return "", fmt.Errorf("unable to validate packaged server app\n%w", err)
	}
	if serverPath == "" {
		return "", nil
	}

	return serverPath, nil
}

func (s ServerBuildSource) ServerPath() (string, error) {
	userPath, err := s.UserPath()
	if err != nil {
		return "", fmt.Errorf("unable to find usr directory\n%w", err)
	}
	if userPath == "" {
		return "", nil
	}

	serverName := s.ServerName
	if serverName == "" {
		serverName, err = s.DefaultServerName()
		if err != nil {
			return "", fmt.Errorf("unable to find server\n%w", err)
		}
	}

	return filepath.Join(userPath, "servers", serverName), nil
}

func (s ServerBuildSource) UserPath() (string, error) {
	dirs := []string{
		filepath.Join("wlp", "usr"),
		"usr",
	}
	for _, dir := range dirs {
		path := filepath.Join(s.InstallRoot, dir)
		exists, err := sherpa.DirExists(path)
		if err != nil {
			return "", nil
		}
		if exists {
			return path, nil
		}
	}

	return "", nil
}
