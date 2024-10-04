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
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/sherpa"
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
		profile == "jakartaee10" ||
		profile == "javaee8" ||
		profile == "webProfile10" ||
		profile == "webProfile8" ||
		profile == "microProfile6" ||
		profile == "microProfile4"
}

func IsValidWebSphereLibertyProfile(profile string) bool {
	return profile == "kernel" ||
		profile == "jakartaee10" ||
		profile == "javaee8" ||
		profile == "javaee7" ||
		profile == "webProfile10" ||
		profile == "webProfile8" ||
		profile == "webProfile7"
}

func GetDefaultFeatures(serverProfile string) []string {
	switch serverProfile {
	case "full", "kernel":
		return []string{"jsp-2.3"}
	case "jakartaee9":
		return []string{"jakartaee-10.0"}
	case "javaee8":
		return []string{"javaee-8.0"}
	case "webProfile9":
		return []string{"webProfile-10.0"}
	case "webProfile8":
		return []string{"webProfile-8.0"}
	case "microProfile5":
		return []string{"microProfile-6.0"}
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

	serverDirs, err := os.ReadDir(serversPath)
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

	if logger.IsDebugEnabled() {
		args = append(args, "--verbose")
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
	Applications           []ApplicationConfig `xml:"application"`
	WebApplications        []ApplicationConfig `xml:"webApplication"`
	EnterpriseApplications []ApplicationConfig `xml:"enterpriseApplication"`
	HTTPEndpoint           struct {
		Host string `xml:"host,attr"`
	} `xml:"httpEndpoint"`
}

type ApplicationConfig struct {
	Id          string `xml:"id,attr"`
	Name        string `xml:"name,attr"`
	Location    string `xml:"location,attr"`
	ContextRoot string `xml:"context-root,attr,omitempty"`
	Type        string `xml:"type,attr,omitempty"`

	AppElement string `xml:"-"`
}

type ApplicationConfigs struct {
	appMap map[string][]ApplicationConfig
}

func ProcessApplicationConfigs(config Config) ApplicationConfigs {
	appMap := make(map[string][]ApplicationConfig)
	for appType, apps := range map[string][]ApplicationConfig{
		"application":           config.Applications,
		"webApplication":        config.WebApplications,
		"enterpriseApplication": config.EnterpriseApplications,
	} {
		for _, app := range apps {
			app.AppElement = appType
			baseApps, ok := appMap[app.Id]
			if !ok {
				baseApps = make([]ApplicationConfig, 0)
			}
			baseApps = append(baseApps, app)
			appMap[app.Id] = baseApps
		}
	}
	return ApplicationConfigs{appMap: appMap}
}

func (apps *ApplicationConfigs) Ids() []string {
	ids := make([]string, 0)
	for key := range apps.appMap {
		ids = append(ids, key)
	}
	return ids
}

func (apps *ApplicationConfigs) HasId(id string) bool {
	_, hasApp := apps.appMap[id]
	return hasApp
}

func (apps *ApplicationConfigs) GetApplication(id string) (ApplicationConfig, error) {
	configs, foundApp := apps.appMap[id]
	if !foundApp {
		return ApplicationConfig{}, fmt.Errorf("unable to find app with ID '%s'", id)
	}

	mergedConfig := ApplicationConfig{
		Id:          id,
		ContextRoot: "/",
	}

	for _, config := range configs {
		if config.Name != "" {
			mergedConfig.Name = config.Name
		}
		if config.Location != "" {
			mergedConfig.Location = config.Location
		}
		if config.ContextRoot != "" {
			mergedConfig.ContextRoot = config.ContextRoot
		}
		if config.Type != "" {
			mergedConfig.Type = config.Type
		}
		if config.AppElement != "" {
			mergedConfig.AppElement = config.AppElement
		}
	}

	return mergedConfig, nil
}

func ReadServerConfig(configPath string) (Config, error) {
	xmlFile, err := os.Open(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("unable to open config '%s'\n%w", configPath, err)
	}
	defer xmlFile.Close()

	content, err := io.ReadAll(xmlFile)
	if err != nil {
		return Config{}, fmt.Errorf("unable to read config '%s'\n%w", configPath, err)
	}

	var config Config
	err = xml.Unmarshal(content, &config)
	if err != nil {
		return Config{}, fmt.Errorf("unable to unmarshal config '%s'\n%w", configPath, err)
	}
	return config, nil
}

type ConfigNode struct {
	node *xmlquery.Node
}

func ReadServerConfigAsNode(configPath string) (ConfigNode, error) {
	config, err := os.ReadFile(configPath)
	if err != nil {
		return ConfigNode{}, fmt.Errorf("unable to read server config\n%w", err)
	}
	reader := bytes.NewReader(config)
	doc, err := xmlquery.Parse(reader)
	if err != nil {
		return ConfigNode{}, fmt.Errorf("unable to parse server config\n%w", err)
	}

	return ConfigNode{
		node: doc,
	}, nil
}

func (n *ConfigNode) GetApplicationNode() (*xmlquery.Node, error) {
	serverNode, err := xmlquery.Query(n.node, "//server")
	if err != nil {
		return nil, fmt.Errorf("unable to find server configuration\n%w", err)
	}
	for _, appElement := range []string{"application", "webApplication", "enterpriseApplication"} {
		appConfig, err := xmlquery.Query(serverNode, "//"+appElement)
		if err != nil {
			return nil, fmt.Errorf("unable to find app configuration '%s'\n%w", appElement, err)
		}
		if appConfig != nil {
			return appConfig, nil
		}
	}
	return nil, nil
}

func (n *ConfigNode) UpdateApplicationId(id string) error {
	appConfig, err := n.GetApplicationNode()
	if err != nil {
		return fmt.Errorf("unable to get application node\n%w", err)
	}
	if appConfig == nil {
		return nil
	}
	appConfig.SetAttr("id", id)
	return nil
}

func (n *ConfigNode) SaveAs(configPath string) error {
	return os.WriteFile(configPath, []byte(n.node.OutputXMLWithOptions(xmlquery.WithOutputSelf(), xmlquery.WithEmptyTagSupport())), 0644)
}
