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

package helper

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/paketo-buildpacks/liberty/internal/server"
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

func (f FileLinker) Configure(appDir string) error {
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
	isPackagedServer, usrPath, err := checkPackagedServer(appDir)
	if err != nil {
		return fmt.Errorf("unable to check package server directory\n%w", err)
	}
	if isPackagedServer {
		libertyServer := server.LibertyServer{
			ServerUserPath: filepath.Join(f.RuntimeRootPath, "usr"),
			ServerName:     serverName,
		}
		if err := libertyServer.SetUserDirectory(usrPath); err != nil {
			return fmt.Errorf("unable to contribute packaged server\n%w", err)
		}
	} else {
		if err = f.ContributeApp(appDir, f.RuntimeRootPath, serverName, b); err != nil {
			return fmt.Errorf("unable to contribute app and config to runtime root\n%w", err)
		}

		if err = f.ContributeDefaultHttpEndpoint(f.RuntimeRootPath, serverName, b); err != nil {
			return fmt.Errorf("unable to contribute default http endpoint\n%w", err)
		}

		if err = f.ContributeUserFeatures(serverName, f.getConfigTemplate(b, "features.tmpl")); err != nil {
			return fmt.Errorf("unable to contribute user features\n%w", err)
		}
	}

	configPath := filepath.Join(f.ServerRootPath, "server.xml")
	if hasBindings {
		if bindingXML, ok := b.SecretFilePath("server.xml"); ok {
			if err = util.DeleteAndLinkPath(bindingXML, configPath); err != nil {
				return fmt.Errorf("unable to replace server.xml\n%w", err)
			}
		}

		if bootstrapProperties, ok := b.SecretFilePath("bootstrap.properties"); ok {
			existingBSP := filepath.Join(f.RuntimeRootPath, "usr", "servers", serverName, "bootstrap.properties")
			if err = util.DeleteAndLinkPath(bootstrapProperties, existingBSP); err != nil {
				return fmt.Errorf("unable to replace bootstrap.properties\n%w", err)
			}
		}
	}

	return nil
}

func (f FileLinker) ContributeApp(appPath, runtimeRoot, serverName string, binding libcnb.Binding) error {
	linkPath := filepath.Join(runtimeRoot, "usr", "servers", serverName, "apps", "app")
	_ = os.Remove(linkPath) // we don't care if this succeeds or fails necessarily, we just want to try to remove anything in the way of the relinking

	if err := os.Symlink(appPath, linkPath); err != nil {
		return fmt.Errorf("unable to symlink application to '%s'\n%w", linkPath, err)
	}

	// Skip contributing app config if already defined in the server.xml
	configPath := filepath.Join(f.ServerRootPath, "server.xml")
	config, err := readServerConfig(configPath)
	if err != nil {
		return fmt.Errorf("unable to read server config\n%w", err)
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

	configOverridesPath := filepath.Join(runtimeRoot, "usr", "servers", serverName, "configDropins", "overrides")
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

func (f FileLinker) ContributeDefaultHttpEndpoint(runtimeRoot, serverName string, binding libcnb.Binding) error {
	configDefaultsPath := filepath.Join(runtimeRoot, "usr", "servers", serverName, "configDropins", "defaults")
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

// checkPackagedServer returns true if a packaged server is detected. If true, it also returns the detected usr path.
func checkPackagedServer(appPath string) (bool, string, error) {
	dirs := []string{
		filepath.Join("wlp", "usr"),
		"usr",
	}

	for _, dir := range dirs {
		userPath := filepath.Join(appPath, dir)
		isPackagedServer, err := util.DirExists(userPath)
		if err != nil {
			return false, "", fmt.Errorf("unable to check user directory\n%w", err)
		}
		if isPackagedServer {
			return true, userPath, nil
		}
	}

	return false, "", nil
}
