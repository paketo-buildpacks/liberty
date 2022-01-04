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
	"github.com/paketo-buildpacks/open-liberty/openliberty"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/bindings"
)

type FileLinker struct {
	Bindings        libcnb.Bindings
	Logger          bard.Logger
	Config          ServerConfig
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
	appDir, ok := os.LookupEnv("BPI_OL_DROPIN_DIR")
	if !ok {
		appDir = "/workspace"
	}

	layerDir, ok := os.LookupEnv("BPI_OL_RUNTIME_ROOT")
	if !ok {
		layerDir = "/layers/paketo-buildpacks_open-liberty/open-liberty-runtime"
	}
	f.RuntimeRootPath = layerDir

	_, err = os.Stat(layerDir)
	if err != nil && os.IsNotExist(err) {
		return map[string]string{}, fmt.Errorf("unable to find '%s', folder does not exist", layerDir)
	} else if err != nil {
		return map[string]string{}, fmt.Errorf("unable to check %s\n%w", layerDir, err)
	}
	if err = f.Configure(layerDir, appDir); err != nil {
		return map[string]string{}, fmt.Errorf("unable to configure\n%w", err)
	}
	return map[string]string{}, nil
}

func (f FileLinker) Configure(layerDir, appDir string) error {
	b, hasBindings, err := bindings.ResolveOne(f.Bindings, bindings.OfType("open-liberty"))
	if err != nil {
		return fmt.Errorf("unable to resolve bindings\n%w", err)
	}

	f.ServerRootPath = filepath.Join(layerDir, "usr", "servers", "defaultServer")
	configPath := filepath.Join(f.ServerRootPath, "server.xml")

	if hasBindings {
		if bindingXML, ok := b.SecretFilePath("server.xml"); ok {
			if err = replaceFile(bindingXML, configPath); err != nil {
				return fmt.Errorf("unable to replace server.xml\n%w", err)
			}
		}

		if bootstrapProperties, ok := b.SecretFilePath("bootstrap.properties"); ok {
			existingBSP := filepath.Join(layerDir, "usr", "servers", "defaultServer", "bootstrap.properties")
			if err = replaceFile(bootstrapProperties, existingBSP); err != nil {
				return fmt.Errorf("unable to replace bootstrap.properties\n%w", err)
			}
		}
	}

	f.Config, err = readServerConfig(configPath)
	if err != nil {
		return fmt.Errorf("unable to read server config\n%w", err)
	}

	f.BaseLayerPath = os.Getenv("BPI_OL_BASE_ROOT")
	if f.BaseLayerPath == "" {
		f.BaseLayerPath = "/layers/paketo-buildpacks_open-liberty/base"
	}

	if err = f.ContributeApp(appDir, layerDir, b); err != nil {
		return fmt.Errorf("unable to contribute app and config to runtime root\n%w", err)
	}

	if err = f.ContributeUserFeatures(b); err != nil {
		return fmt.Errorf("unable to contribute user features: %w", err)
	}

	return nil
}

func (f FileLinker) ContributeApp(appPath, runtimeRoot string, binding libcnb.Binding) error {
	linkPath := filepath.Join(runtimeRoot, "usr", "servers", "defaultServer", "apps", "app")
	_ = os.Remove(linkPath) // we don't care if this succeeds or fails necessarily, we just want to try to remove anything in the way of the relinking
	if err := os.Symlink(appPath, linkPath); err != nil {
		return fmt.Errorf("unable to symlink application to '%s'\n%w", linkPath, err)
	}

	// Skip contributing app config if already defined in the server.xml
	if f.Config.Application.Name == "app" {
		f.Logger.Debugf("server.xml already has an application named 'app' defined. Skipping contribution of app config snippet...")
		return nil
	}

	contextRoot := os.Getenv("BP_OPENLIBERTY_CONTEXT_ROOT")
	if contextRoot == "" {
		contextRoot = "/"
	}

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

	configOverridesPath := filepath.Join(runtimeRoot, "usr", "servers", "defaultServer", "configDropins", "overrides")
	if err := os.MkdirAll(configOverridesPath, 0755); err != nil {
		return fmt.Errorf("unable to make config overrides directory\n%w", err)
	}

	appConfigPath := filepath.Join(configOverridesPath, "app.xml")
	file, err := os.Create(appConfigPath)
	if err != nil {
		return fmt.Errorf("unable to create file '%v'\n%w", appConfig, err)
	}
	defer file.Close()
	err = t.Execute(file, appConfig)
	if err != nil {
		return fmt.Errorf("unable to execute template\n%w", err)
	}
	return nil
}

func (f FileLinker) ContributeUserFeatures(binding libcnb.Binding) error {
	confDir := filepath.Join(f.BaseLayerPath, "conf")

	runtimeLibsPath := filepath.Join(f.RuntimeRootPath, "usr", "extension", "lib")
	runtimeFeaturesPath := filepath.Join(runtimeLibsPath, "features")
	err := os.MkdirAll(runtimeFeaturesPath, 0755)
	if err != nil {
		return err
	}

	featureDescriptor, err := openliberty.ReadFeatureDescriptor(confDir, f.Logger)
	if err != nil {
		return err
	}

	if len(featureDescriptor.Features) <= 0 {
		return nil
	}

	if err = featureDescriptor.ResolveFeatures(); err != nil {
		return err
	}

	var featuresToEnable []string
	for _, feature := range featureDescriptor.Features {
		// Link the feature into place
		featureBase := filepath.Base(feature.ResolvedPath)
		if err := replaceFile(feature.ResolvedPath, filepath.Join(runtimeLibsPath, featureBase)); err != nil {
			return fmt.Errorf("unable to link feature '%v':\n%w", feature.Name, err)
		}
		if feature.ManifestPath != "" {
			manifestBase := filepath.Base(feature.ManifestPath)
			if err := replaceFile(feature.ManifestPath, filepath.Join(runtimeFeaturesPath, manifestBase)); err != nil {
				return fmt.Errorf("unable to link feature manifest for '%v':\n%w", feature.Name, err)
			}
		}
		featuresToEnable = append(featuresToEnable, "usr:"+feature.Name)
		if len(feature.Dependencies) > 0 {
			featuresToEnable = append(featuresToEnable, feature.Dependencies...)
		}
	}

	templatePath := f.getConfigTemplate(binding, "features.tmpl")
	t, err := template.New("features.tmpl").ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("unable to create features template:\n%w", err)
	}
	featuresConfigPath := filepath.Join(f.ServerRootPath, "configDropins", "overrides", "features.xml")
	file, err := os.Create(featuresConfigPath)
	if err != nil {
		return fmt.Errorf("unable to create file '%v':\n%w", featuresConfigPath, err)
	}
	defer file.Close()
	err = t.Execute(file, featuresToEnable)
	if err != nil {
		return fmt.Errorf("unable to execute template:\n%w", err)
	}

	return nil
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

func replaceFile(from, to string) error {
	if _, err := os.Stat(from); err != nil {
		return fmt.Errorf("unable to find file '%s'\n%w", from, err)
	}
	if err := os.Remove(to); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to delete original file '%s'\n%w", from, err)
	}
	if err := os.Symlink(from, to); err != nil {
		return fmt.Errorf("unable to symlink file from '%s' to '%s'\n%w", from, to, err)
	}
	return nil
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
