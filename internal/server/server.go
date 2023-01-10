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
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/sherpa"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func GetServerConfigPath(serverPath string) string {
	return filepath.Join(serverPath, "server.xml")
}

func GetServerConfigs(serverPath string) ([]string, error) {
	configs := []string{}

	serverConfigPath := GetServerConfigPath(serverPath)
	exists, err := sherpa.FileExists(serverConfigPath)
	if err != nil {
		return nil, fmt.Errorf("unable to check for server config\n%w", err)
	}
	if exists {
		configs = append(configs, serverConfigPath)
	}

	defaultConfigs, err := util.GetFiles(filepath.Join(serverPath, "configDropins", "defaults"), "*.xml")
	if err != nil {
		return nil, fmt.Errorf("unable to list default configs\n%w", err)
	}
	configs = append(configs, defaultConfigs...)

	overrideConfigs, err := util.GetFiles(filepath.Join(serverPath, "configDropins", "overrides"), "*.xml")
	if err != nil {
		return nil, fmt.Errorf("unable to list override configs\n%w", err)
	}
	configs = append(configs, overrideConfigs...)

	return configs, nil
}

func GetFeatureList(profile string, serverPath string, additionalFeatures []string) ([]string, error) {
	featureMap := make(map[string]bool, 0)

	// Add any additional features
	for _, feature := range additionalFeatures {
		featureMap[feature] = true
	}

	configs, err := GetServerConfigs(serverPath)
	if err != nil {
		return nil, fmt.Errorf("unable to get server configs\n%w", err)
	}
	for _, configPath := range configs {
		config, err := ReadServerConfig(configPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read config\n%w", err)
		}
		for _, feature := range config.FeatureManager.Features {
			featureMap[feature] = true
		}
	}

	if len(featureMap) == 0 {
		return GetDefaultFeatures(profile), nil
	}

	var features []string
	for feature := range featureMap {
		features = append(features, feature)
	}

	sort.Strings(features)
	return features, nil
}

func IsValidOpenLibertyProfile(profile string) bool {
	return profile == "full" ||
		profile == "kernel" ||
		profile == "jakartaee9" ||
		profile == "javaee8" ||
		profile == "webProfile9" ||
		profile == "webProfile8" ||
		profile == "microProfile5" ||
		profile == "microProfile4"
}

func IsValidWebSphereLibertyProfile(profile string) bool {
	return profile == "kernel" ||
		profile == "jakartaee9" ||
		profile == "javaee8" ||
		profile == "javaee7" ||
		profile == "webProfile9" ||
		profile == "webProfile8" ||
		profile == "webProfile7"
}

func GetDefaultFeatures(serverProfile string) []string {
	switch serverProfile {
	case "full", "kernel":
		return []string{"jsp-2.3"}
	case "jakartaee9":
		return []string{"jakartaee-9.1"}
	case "javaee8":
		return []string{"javaee-8.0"}
	case "webProfile9":
		return []string{"webProfile-9.1"}
	case "webProfile8":
		return []string{"webProfile-8.0"}
	case "microProfile5":
		return []string{"microProfile-5.0"}
	case "microProfile4":
		return []string{"microProfile-4.1"}
	}
	return []string{}
}

// SetUserDirectory sets the server's user directory to the specified directory.
func SetUserDirectory(srcUserPath string, destUserPath string, serverName string) error {
	// Copy the configDropins directory to the new user directory. This is needed by Liberty runtimes provided in the
	// stack run image
	configDropinsDir := filepath.Join(destUserPath, "servers", "defaultServer", "configDropins")
	if configDropinsFound, err := sherpa.DirExists(configDropinsDir); err != nil {
		return fmt.Errorf("unable to read configDropins directory\n%w", err)
	} else if configDropinsFound {
		newConfigDropinsDir := filepath.Join(srcUserPath, "servers", serverName, "configDropins")
		if err := sherpa.CopyDir(configDropinsDir, newConfigDropinsDir); err != nil {
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
	if hasApps, err := util.HasCompiledArtifacts(filepath.Join(serverPath, "apps")); err != nil {
		return false, fmt.Errorf("unable to check apps directory for app archives\n%w", err)
	} else if hasApps {
		return true, nil
	}

	if hasDropins, err := util.HasCompiledArtifacts(filepath.Join(serverPath, "dropins")); err != nil {
		return false, fmt.Errorf("unable to check dropins directory for app archives\n%w", err)
	} else if hasDropins {
		return true, nil
	}

	return false, nil
}

func GetServerList(userPath string) ([]string, error) {
	serversPath := filepath.Join(userPath, "servers")

	serversExists, err := sherpa.DirExists(serversPath)
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
		if !strings.HasPrefix(dir.Name(), ".") && dir.IsDir() {
			servers = append(servers, dir.Name())
		}
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

func InstallFeatures(runtimePath string, serverName string, executor effect.Executor, logger bard.Logger) error {
	logger.Bodyf("Installing features...")

	args := []string{
		"installServerFeatures",
		"--acceptLicense",
		"--noCache",
		serverName,
	}

	if err := executor.Execute(effect.Execution{
		Command: filepath.Join(runtimePath, "bin", "featureUtility"),
		Args:    args,
		Stdout:  bard.NewWriter(logger.InfoWriter(), bard.WithIndent(3)),
		Stderr:  bard.NewWriter(logger.InfoWriter(), bard.WithIndent(3)),
	}); err != nil {
		return fmt.Errorf("unable to install feature\n%w", err)
	}

	return nil
}

type InstalledIFix struct {
	APAR string
	IFix string
}

func GetInstalledIFixes(runtimePath string, executor effect.Executor) ([]InstalledIFix, error) {
	buf := &bytes.Buffer{}

	if err := executor.Execute(effect.Execution{
		Command: filepath.Join(runtimePath, "bin", "productInfo"),
		Args:    []string{"version", "--ifixes"},
		Stdout:  buf,
	}); err != nil {
		return []InstalledIFix{}, fmt.Errorf("unable to get installed iFixes\n%w", err)
	}

	re, err := regexp.Compile(`^(.*) in the iFix\(es\): \[(.*)\]$`)
	if err != nil {
		return []InstalledIFix{}, fmt.Errorf("unable to create iFix regex\n%w", err)
	}

	installed := make([]InstalledIFix, 0)
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := scanner.Text()
		if match := re.FindStringSubmatch(line); len(match) >= 2 {
			installed = append(installed, InstalledIFix{
				APAR: strings.TrimSpace(match[1]),
				IFix: strings.TrimSpace(match[2]),
			})
		}
	}

	return installed, nil
}

func GetInstalledFeatures(runtimePath string, executor effect.Executor) ([]string, error) {
	buf := &bytes.Buffer{}

	if err := executor.Execute(effect.Execution{
		Command: filepath.Join(runtimePath, "bin", "productInfo"),
		Args:    []string{"featureInfo"},
		Stdout:  buf,
	}); err != nil {
		return []string{}, fmt.Errorf("unable to get list of installed features\n%w", err)
	}

	installedFeatures := make([]string, 0)
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		feature := strings.TrimSpace(scanner.Text())
		if len(feature) <= 0 {
			continue
		}
		installedFeatures = append(installedFeatures, feature)
	}

	return installedFeatures, nil
}

type Config struct {
	XMLName        xml.Name `xml:"server"`
	FeatureManager struct {
		Features []string `xml:"feature"`
	} `xml:"featureManager"`
	Application struct {
		Name string `xml:"name,attr"`
	} `xml:"application"`
	HTTPEndpoint struct {
		Host string `xml:"host,attr"`
	} `xml:"httpEndpoint"`
}

func ReadServerConfig(configPath string) (Config, error) {
	xmlFile, err := os.Open(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("unable to open config '%s'\n%w", configPath, err)
	}
	defer xmlFile.Close()

	bytes, err := ioutil.ReadAll(xmlFile)
	if err != nil {
		return Config{}, fmt.Errorf("unable to read config '%s'\n%w", configPath, err)
	}

	var config Config
	err = xml.Unmarshal(bytes, &config)
	if err != nil {
		return Config{}, fmt.Errorf("unable to unmarshal config '%s'\n%w", configPath, err)
	}
	return config, nil
}
