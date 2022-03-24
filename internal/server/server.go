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

package server

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
)

func GetServerConfigPath(serverPath string) string {
	return filepath.Join(serverPath, "server.xml")
}

// SetUserDirectory sets the server's user directory to the specified directory.
func SetUserDirectory(srcUserPath string, destUserPath string, serverName string) error {
	// Copy the configDropins directory to the new user directory. This is needed by Liberty runtimes provided in the
	// stack run image
	configDropinsDir := filepath.Join(destUserPath, "servers", "defaultServer", "configDropins")
	if configDropinsFound, err := util.DirExists(configDropinsDir); err != nil {
		return fmt.Errorf("unable to read configDropins directory\n%w", err)
	} else if configDropinsFound {
		newConfigDropinsDir := filepath.Join(srcUserPath, "servers", serverName, "configDropins")
		if err := util.CopyDir(configDropinsDir, newConfigDropinsDir); err != nil {
			return fmt.Errorf("unable to copy configDropins to new user directory\n%w", err)
		}
	}
	if err := util.DeleteAndLinkPath(srcUserPath, destUserPath); err != nil {
		return fmt.Errorf("unable to set new user directory\n%w", err)
	}
	return nil
}

// HasInstalledApps checks the directories `<server-path>/apps` and `<server-path>/dropins` for any web or enterprise
// archives. Returns true if it finds at least one compiled artifact.
func HasInstalledApps(serverPath string) (bool, error) {
	if hasApps, err := dirHasCompiledArtifacts(filepath.Join(serverPath, "apps")); err != nil {
		return false, fmt.Errorf("unable to check apps directory for app archives\n%w", err)
	} else if hasApps {
		return true, nil
	}

	if hasDropins, err := dirHasCompiledArtifacts(filepath.Join(serverPath, "dropins")); err != nil {
		return false, fmt.Errorf("unable to check dropins directory for app archives\n%w", err)
	} else if hasDropins {
		return true, nil
	}

	return false, nil
}

func GetServerList(userPath string) ([]string, error) {
	serversPath := filepath.Join(userPath, "servers")

	serversExists, err := util.FileExists(serversPath)
	if err != nil {
		return []string{}, err
	}
	if !serversExists {
		return []string{}, nil
	}

	serverDirs, err := ioutil.ReadDir(serversPath)
	if err != nil {
		return []string{}, err
	}

	var servers []string
	for _, dir := range serverDirs {
		servers = append(servers, dir.Name())
	}

	return servers, nil
}

func LoadIFixesList(ifixesPath string) ([]string, error) {
	return filepath.Glob(fmt.Sprintf("%s/*.jar", ifixesPath))
}

func InstallIFixes(installRoot string, ifixes []string, executor effect.Executor, logger bard.Logger) error {
	for _, ifix := range ifixes {
		logger.Bodyf("Installing iFix %s\n", filepath.Base(ifix))
		if err := executor.Execute(effect.Execution{
			Command: "java",
			Args:    []string{"-jar", ifix, "--installLocation", installRoot},
			Stdout:  bard.NewWriter(logger.InfoWriter(), bard.WithIndent(3)),
			Stderr:  bard.NewWriter(logger.InfoWriter(), bard.WithIndent(3)),
		}); err != nil {
			return fmt.Errorf("unable to install iFix '%s'\n%w", filepath.Base(ifix), err)
		}
	}

	return nil
}

func InstallFeatures(installRoot string, features []string, executor effect.Executor, logger bard.Logger) error {
	if len(features) > 0 {
		logger.Bodyf("Installing features with arguments %s\n", features)

		args := []string{"installFeature"}
		args = append(args, features...)
		args = append(args, "--acceptLicense")

		if err := executor.Execute(effect.Execution{
			Command: filepath.Join(installRoot, "bin", "featureUtility"),
			Args:    args,
			Stdout:  bard.NewWriter(logger.InfoWriter(), bard.WithIndent(3)),
			Stderr:  bard.NewWriter(logger.InfoWriter(), bard.WithIndent(3)),
		}); err != nil {
			return fmt.Errorf("unable to install feature '%s'\n%w", features, err)
		}
	}

	return nil
}

// dirHasCompiledArtifacts checks if the given directory has any web or enterprise archives.
func dirHasCompiledArtifacts(path string) (bool, error) {
	if exists, err := util.DirExists(path); err != nil {
		return false, err
	} else if !exists {
		return false, nil
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return false, err
	}
	for _, file := range files {
		if name := file.Name(); strings.HasSuffix(name, ".war") || strings.HasSuffix(name, ".ear") {
			return true, nil
		}
	}
	return false, nil
}
