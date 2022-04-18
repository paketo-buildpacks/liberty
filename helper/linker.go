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

package helper

import (
	"encoding/xml"
	"fmt"
	"github.com/paketo-buildpacks/liberty/internal/core"
	"github.com/paketo-buildpacks/liberty/internal/server"
	"github.com/paketo-buildpacks/libpak/crush"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/paketo-buildpacks/liberty/liberty"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/bindings"
	"github.com/paketo-buildpacks/libpak/sherpa"
)

type FileLinker struct {
	Bindings        libcnb.Bindings
	Logger          bard.Logger
	BaseLayerPath   string
	RuntimeRootPath string
	ServerRootPath  string
}

type ApplicationConfig struct {
	Path        string
	ContextRoot string
	Type        string
}

type ServerConfig struct {
	XMLName     xml.Name `xml:"server"`
	Application struct {
		XMLName xml.Name `xml:"application"`
		Name    string   `xml:"name,attr"`
	} `xml:"application"`
	HTTPEndpoint struct {
		XMLName xml.Name `xml:"httpEndpoint"`
		Host    string   `xml:"host,attr"`
	} `xml:"httpEndpoint"`
}

func (f FileLinker) Execute() (map[string]string, error) {
	var err error
	appDir := sherpa.GetEnvWithDefault("BPI_LIBERTY_DROPIN_DIR", "/workspace")
	f.RuntimeRootPath, err = sherpa.GetEnvRequired("BPI_LIBERTY_RUNTIME_ROOT")
	if err != nil {
		return map[string]string{}, err
	}

	_, err = os.Stat(f.RuntimeRootPath)
	if err != nil && os.IsNotExist(err) {
		return map[string]string{}, fmt.Errorf("unable to find '%s', folder does not exist", f.RuntimeRootPath)
	} else if err != nil {
		return map[string]string{}, fmt.Errorf("unable to check %s\n%w", f.RuntimeRootPath, err)
	}
	if err = f.Configure(appDir); err != nil {
		return map[string]string{}, fmt.Errorf("unable to configure\n%w", err)
	}
	return map[string]string{}, nil
}

func (f FileLinker) Configure(workspacePath string) error {
	b, hasBindings, err := bindings.ResolveOne(f.Bindings, bindings.OfType("liberty"))
	if err != nil {
		return fmt.Errorf("unable to resolve bindings\n%w", err)
	}

	serverName, err := sherpa.GetEnvRequired("BPI_LIBERTY_SERVER_NAME")
	if err != nil {
		return err
	}

	f.ServerRootPath = filepath.Join(f.RuntimeRootPath, "usr", "servers", serverName)
	f.BaseLayerPath = sherpa.GetEnvWithDefault("BPI_LIBERTY_BASE_ROOT", "/layers/paketo-buildpacks_liberty/base")

	// Check if we are contributing a packaged server
	serverBuildSource := core.NewServerBuildSource(workspacePath, serverName, f.Logger)
	isPackagedServer, err := serverBuildSource.Detect()
	if err != nil {
		return fmt.Errorf("unable to check package server directory\n%w", err)
	}
	if isPackagedServer {
		usrPath, err := serverBuildSource.UserPath()
		if err != nil {
			return fmt.Errorf("unable to get Liberty usr directory\n%w", err)
		}
		destUserPath := filepath.Join(f.RuntimeRootPath, "usr")
		if err := server.SetUserDirectory(usrPath, destUserPath, serverName); err != nil {
			return fmt.Errorf("unable to contribute packaged server\n%w", err)
		}
		return nil
	}

	// Contribute server config files if found in workspace
	configs := []string{
		"server.xml",
		"server.env",
		"bootstrap.properties",
		"jvm.options",
	}

	for _, config := range configs {
		configPath := filepath.Join(workspacePath, config)
		configExists, err := util.FileExists(configPath)
		if err != nil {
			return err
		}
		if configExists {
			toPath := filepath.Join(f.ServerRootPath, config)
			if err = util.DeleteAndLinkPath(configPath, toPath); err != nil {
				return fmt.Errorf("unable to copy config from workspace\n%w", err)
			}
		}
	}

	if hasBindings {
		if bindingXML, ok := b.SecretFilePath("server.xml"); ok {
			if err = util.DeleteAndLinkPath(bindingXML, filepath.Join(f.ServerRootPath, "server.xml")); err != nil {
				return fmt.Errorf("unable to replace server.xml\n%w", err)
			}
		}

		if bootstrapProperties, ok := b.SecretFilePath("bootstrap.properties"); ok {
			existingBSP := filepath.Join(f.ServerRootPath, "bootstrap.properties")
			if err = util.DeleteAndLinkPath(bootstrapProperties, existingBSP); err != nil {
				return fmt.Errorf("unable to replace bootstrap.properties\n%w", err)
			}
		}
	}

	// Skip contributing app config if already defined in the server.xml
	configPath := filepath.Join(f.ServerRootPath, "server.xml")
	config, err := readServerConfig(configPath)
	if err != nil {
		return fmt.Errorf("unable to read server config\n%w", err)
	}

	if err = f.ContributeApp(workspacePath, config, b); err != nil {
		return fmt.Errorf("unable to contribute app and config to runtime root\n%w", err)
	}

	if err = f.ContributeDefaultHttpEndpoint(config, b); err != nil {
		return fmt.Errorf("unable to contribute default http endpoint\n%w", err)
	}

	if err = f.ContributeUserFeatures(serverName, f.getConfigTemplate(b, "features.tmpl")); err != nil {
		return fmt.Errorf("unable to contribute user features\n%w", err)
	}

	return nil
}

func (f FileLinker) ContributeApp(workspacePath string, config ServerConfig, binding libcnb.Binding) error {
	// Determine app path
	var appPath string
	if appPaths, err := util.GetApps(workspacePath); err != nil {
		return fmt.Errorf("unable to determine apps to contribute\n%w", err)
	} else if len(appPaths) == 0 {
		appPath = workspacePath
	} else if len(appPaths) == 1 {
		appPath = appPaths[0]
	} else {
		return fmt.Errorf("expected one app but found several: %s", strings.Join(appPaths, ","))
	}

	linkPath := filepath.Join(f.ServerRootPath, "apps", "app")
	_ = os.Remove(linkPath) // we don't care if this succeeds or fails necessarily, we just want to try to remove anything in the way of the relinking

	// Expand app if needed
	isDir, err := util.DirExists(appPath)
	if err != nil {
		return fmt.Errorf("unable to check if app path is a directory\n%w", err)
	}
	if isDir {
		if err := os.Symlink(appPath, linkPath); err != nil {
			return fmt.Errorf("unable to symlink application to '%s'\n%w", linkPath, err)
		}
	} else {
		compiledArtifact, err := os.Open(appPath)
		if err != nil {
			return fmt.Errorf("unable to open compiled artifact\n%w", err)
		}
		err = crush.Extract(compiledArtifact, linkPath, 0)
		if err != nil {
			return fmt.Errorf("unable to extract compiled artifact\n%w", err)
		}
	}

	if config.Application.Name == "app" {
		f.Logger.Debugf("server.xml already has an application named 'app' defined. Skipping contribution of app config snippet...")
		return nil
	}

	contextRoot := sherpa.GetEnvWithDefault("BP_LIBERTY_CONTEXT_ROOT", "/")
	appType := "war"
	if _, err := os.Stat(filepath.Join(appPath, "META-INF", "application.xml")); err == nil {
		appType = "ear"
	}

	appConfig := ApplicationConfig{
		Path:        linkPath,
		ContextRoot: contextRoot,
		Type:        appType,
	}

	templatePath := f.getConfigTemplate(binding, "app.tmpl")
	t, err := template.New("app.tmpl").ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("unable to create app template\n%w", err)
	}

	configOverridesPath := filepath.Join(f.ServerRootPath, "configDropins", "overrides")
	if err := os.MkdirAll(configOverridesPath, 0755); err != nil {
		return fmt.Errorf("unable to make config overrides directory\n%w", err)
	}

	appConfigPath := filepath.Join(configOverridesPath, "app.xml")
	file, err := os.Create(appConfigPath)
	if err != nil {
		return fmt.Errorf("unable to create file '%s'\n%w", appConfig, err)
	}
	defer file.Close()
	err = t.Execute(file, appConfig)
	if err != nil {
		return fmt.Errorf("unable to execute template\n%w", err)
	}
	return nil
}

func (f FileLinker) ContributeUserFeatures(serverName, configTemplatePath string) error {
	confDir := filepath.Join(f.BaseLayerPath, "conf")
	featureDescriptor, err := liberty.ReadFeatureDescriptor(confDir, f.Logger)
	if err != nil {
		return err
	}

	if len(featureDescriptor.Features) <= 0 {
		return nil
	}

	runtimeLibsPath := filepath.Join(f.RuntimeRootPath, "usr", "extension", "lib")
	runtimeFeaturesPath := filepath.Join(runtimeLibsPath, "features")
	if err := os.MkdirAll(runtimeFeaturesPath, 0755); err != nil {
		return err
	}

	if err = featureDescriptor.ResolveFeatures(); err != nil {
		return err
	}

	featureInstaller := liberty.NewFeatureInstaller(
		f.RuntimeRootPath,
		serverName,
		configTemplatePath,
		featureDescriptor.Features)

	if err := featureInstaller.Install(); err != nil {
		return err
	}
	if err := featureInstaller.Enable(); err != nil {
		return err
	}

	return nil
}

func (f FileLinker) ContributeDefaultHttpEndpoint(config ServerConfig, binding libcnb.Binding) error {
	if config.HTTPEndpoint.Host != "" {
		f.Logger.Debugf("server.xml already has an httpEndpoint defined; skipping contribution of default HTTP endpoint config snippet...")
		return nil
	}
	configDefaultsPath := filepath.Join(f.ServerRootPath, "configDropins", "defaults")
	if err := os.MkdirAll(configDefaultsPath, 0755); err != nil {
		return fmt.Errorf("unable to make config defaults directory\n%w", err)
	}
	templatePath := f.getConfigTemplate(binding, "default-http-endpoint.tmpl")
	configPath := filepath.Join(configDefaultsPath, "default-http-endpoint.xml")
	return util.CopyFile(templatePath, configPath)
}

func (f FileLinker) getConfigTemplate(binding libcnb.Binding, template string) string {
	// Get customized template if it has been provided
	if binding, ok := binding.SecretFilePath(template); ok {
		f.Logger.Bodyf("Using custom template: %s", binding)
		return binding
	}
	// Use default config template
	return filepath.Join(f.BaseLayerPath, "templates", template)
}

func readServerConfig(configPath string) (ServerConfig, error) {
	xmlFile, err := os.Open(configPath)
	if err != nil {
		return ServerConfig{}, fmt.Errorf("unable to open server.xml '%s'\n%w", configPath, err)
	}
	defer xmlFile.Close()

	bytes, err := ioutil.ReadAll(xmlFile)
	if err != nil {
		return ServerConfig{}, fmt.Errorf("unable to read server.xml '%s'\n%w", configPath, err)
	}

	var config ServerConfig
	err = xml.Unmarshal(bytes, &config)
	if err != nil {
		return ServerConfig{}, fmt.Errorf("unable to unmarshal server.xml: '%s'\n%w", configPath, err)
	}
	return config, nil
}
