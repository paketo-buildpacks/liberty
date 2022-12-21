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
	"errors"
	"fmt"
	"github.com/paketo-buildpacks/liberty/internal/core"
	"github.com/paketo-buildpacks/liberty/internal/server"
	"github.com/paketo-buildpacks/libpak/crush"
	"github.com/paketo-buildpacks/libpak/sherpa"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/buildpacks/libcnb"
	"github.com/heroku/color"
	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
)

type Base struct {
	ApplicationPath       string
	BuildpackPath         string
	LayerContributor      libpak.LayerContributor
	Logger                bard.Logger
	ServerName            string
	Features              []string
	ContextRoot           string
	UserFeatureDescriptor *FeatureDescriptor
	LibertyBinding        libcnb.Binding
	JVM                   string
}

func NewBase(
	appPath string,
	buildpackPath string,
	serverName string,
	features []string,
	contextRoot string,
	userFeatureDescriptor *FeatureDescriptor,
	libertyBinding libcnb.Binding,
	logger bard.Logger,
	jvmName string,
) Base {
	workspaceSum, err := sherpa.NewFileListingHash(appPath)
	if err != nil {
		logger.Info(color.RedString("unable to calculate checksum of workspace directory\n%w", err))
	}
	var enabledUserFeatures []string
	for _, feature := range userFeatureDescriptor.Features {
		enabledUserFeatures = append(enabledUserFeatures, fmt.Sprintf("%s-%s", feature.Name, feature.Version))
	}

	expectedMetadata := map[string]interface{}{
		"serverName":   serverName,
		"features":     features,
		"contextRoot":  contextRoot,
		"userFeatures": enabledUserFeatures,
		"workspaceSum": workspaceSum,
	}

	// Add checksum for /templates if provided
	templatesSum, err := getTemplatesChecksum()
	if err == nil {
		if templatesSum != "" {
			expectedMetadata["templatesSum"] = templatesSum
		}
	} else {
		logger.Info(color.RedString("unable to get checksum for templates\n%w", err))
	}

	contributor := libpak.NewLayerContributor(
		"Open Liberty Config",
		expectedMetadata,
		libcnb.LayerTypes{
			Launch: true,
		})

	return Base{
		ApplicationPath:       appPath,
		BuildpackPath:         buildpackPath,
		LayerContributor:      contributor,
		ServerName:            serverName,
		Features:              features,
		ContextRoot:           contextRoot,
		UserFeatureDescriptor: userFeatureDescriptor,
		LibertyBinding:        libertyBinding,
		Logger:                logger,
		JVM:                   jvmName,
	}
}

func (b Base) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	b.LayerContributor.Logger = b.Logger

	return b.LayerContributor.Contribute(layer, func() (libcnb.Layer, error) {
		err := b.contribute(layer)
		if err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to contribute base\n%w", err)
		}
		return layer, nil
	})
}

func (b Base) contribute(layer libcnb.Layer) error {
	layer.LaunchEnvironment.Default("BPI_LIBERTY_SERVER_NAME", b.ServerName)

	// OpenJ9 only:
	// Enable verbose GC logging by default as strongly recommended by Liberty support. It is low overhead and helps
	// diagnose any high heap issues.
	if b.JVM == "OpenJ9" {
		layer.LaunchEnvironment.Appendf("JAVA_TOOL_OPTIONS", " ", "-Xverbosegclog:verbosegc.%%pid.%%seq.log,5,10000")
	}

	serverBuildSource := core.NewServerBuildSource(b.ApplicationPath, b.ServerName, b.Logger)
	isPackagedServer, err := serverBuildSource.Detect()
	if err != nil {
		return fmt.Errorf("unable to check package server directory\n%w", err)
	}
	if isPackagedServer {
		usrPath, err := serverBuildSource.UserPath()
		if err != nil {
			return fmt.Errorf("unable to get Liberty user directory\n%w", err)
		}
		layer.LaunchEnvironment.Default("WLP_USER_DIR", usrPath)
		if err := os.Setenv("WLP_USER_DIR", usrPath); err != nil {
			return fmt.Errorf("unable to set WLP_USER_DIR for packaged server\n%w", err)
		}
		return nil
	}

	serverPath := filepath.Join(layer.Path, "wlp", "usr", "servers", b.ServerName)
	err = b.createServerDirectory(layer)
	if err != nil {
		return fmt.Errorf("unable to create server directory\n%w", err)
	}
	layer.LaunchEnvironment.Default("WLP_USER_DIR", filepath.Join(layer.Path, "wlp", "usr"))
	if err := os.Setenv("WLP_USER_DIR", filepath.Join(layer.Path, "wlp", "usr")); err != nil {
		return fmt.Errorf("unable to set WLP_USER_DIR\n%w", err)
	}

	// Contribute server config files if found in workspace
	configs := []string{
		"server.xml",
		"server.env",
		"bootstrap.properties",
	}

	for _, config := range configs {
		configPath := filepath.Join(b.ApplicationPath, config)
		toPath := filepath.Join(serverPath, config)
		err = util.DeleteAndLinkPath(configPath, toPath)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("unable to copy config from workspace\n%w", err)
			}
			continue
		}
		b.Logger.Info(color.YellowString("Reminder: Do not include secrets in %s; this file has been included in the image and that can leak your secrets", config))
	}

	err = b.contributeConfig(serverPath)
	if err != nil {
		return fmt.Errorf("unable to contribute config\n%w", err)
	}

	if err := b.contributeUserFeatures(layer); err != nil {
		return fmt.Errorf("unable to contribute user features\n%w", err)
	}

	configPath := filepath.Join(serverPath, "server.xml")
	config, err := server.ReadServerConfig(configPath)
	if err != nil {
		return fmt.Errorf("unable to read server config\n%w", err)
	}

	err = b.contributeApp(layer, config)
	if err != nil {
		return fmt.Errorf("unable to contribute config\n%w", err)
	}

	return nil
}

func (b Base) createServerDirectory(layer libcnb.Layer) error {
	serverPath := filepath.Join(layer.Path, "wlp", "usr", "servers", b.ServerName)
	serverDirs := []string{
		"apps",
		filepath.Join("configDropins", "overrides"),
		filepath.Join("configDropins", "defaults"),
	}

	for _, serverDir := range serverDirs {
		if err := os.MkdirAll(filepath.Join(serverPath, serverDir), 0744); err != nil {
			return fmt.Errorf("unable to create server directory\n%w", err)
		}
	}

	return filepath.Walk(filepath.Join(layer.Path, "wlp"), func(path string, _ os.FileInfo, err error) error {
		return os.Chmod(path, 0755)
	})
}

func (b Base) contributeConfig(serverPath string) error {
	serverConfigPath := filepath.Join(serverPath, "server.xml")
	exists, err := sherpa.FileExists(serverConfigPath)
	if err != nil {
		return fmt.Errorf("unable to check if server.xml exists\n%w", err)
	}
	if !exists {
		templatePath, err := b.getConfigTemplate("server.tmpl")
		if err != nil {
			return fmt.Errorf("unable to get server config template\n%w", err)
		}
		t, err := template.New("server.tmpl").ParseFiles(templatePath)
		if err != nil {
			return fmt.Errorf("unable to create server template\n%w", err)
		}

		file, err := os.Create(serverConfigPath)
		if err != nil {
			return fmt.Errorf("unable to create file '%s'\n%w", serverConfigPath, err)
		}
		defer file.Close()
		err = t.Execute(file, b.Features)
		if err != nil {
			return fmt.Errorf("unable to execute template\n%w", err)
		}
	}

	endpointTemplate, err := b.getConfigTemplate("expose-default-endpoint.xml")
	if err != nil {
		return fmt.Errorf("unable to get endpoint config\n%w", err)
	}
	file, err := os.Open(endpointTemplate)
	if err != nil {
		return fmt.Errorf("unable to open endpoint config\n%w", err)
	}
	err = sherpa.CopyFile(file, filepath.Join(serverPath, "configDropins", "defaults", "expose-default-endpoint.xml"))
	if err != nil {
		return fmt.Errorf("unable to copy endpoint config\n%w", err)
	}
	return nil
}

func (b Base) contributeApp(layer libcnb.Layer, config server.Config) error {
	// Determine app path
	var appPath string
	if appPaths, err := util.GetApps(b.ApplicationPath); err != nil {
		return fmt.Errorf("unable to determine apps to contribute\n%w", err)
	} else if len(appPaths) == 0 {
		appPath = b.ApplicationPath
	} else if len(appPaths) == 1 {
		appPath = appPaths[0]
	} else {
		return fmt.Errorf("expected one app but found several: %s", strings.Join(appPaths, ","))
	}

	serverPath := filepath.Join(layer.Path, "wlp", "usr", "servers", b.ServerName)

	linkPath := filepath.Join(serverPath, "apps", "app")
	if err := os.RemoveAll(linkPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("unable to remove app\n%w", err)
	}

	// Expand app if needed
	isDir, err := sherpa.DirExists(appPath)
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
		if err := crush.Extract(compiledArtifact, linkPath, 0); err != nil {
			return fmt.Errorf("unable to extract compiled artifact\n%w", err)
		}
		if err := os.Remove(appPath); err != nil {
			return fmt.Errorf("unable to remove compiled artifact\n%w", err)
		}
	}

	appType := "war"
	if _, err := os.Stat(filepath.Join(appPath, "META-INF", "application.xml")); err == nil {
		appType = "ear"
	}
	if err := b.createAppConfig(serverPath, linkPath, b.ContextRoot, appType, config); err != nil {
		return fmt.Errorf("unable to create app config\n%w", err)
	}
	return nil
}

func (b Base) createAppConfig(serverPath string, appPath string, contextRoot string, appType string, config server.Config) error {
	appConfigs := server.ProcessApplicationConfigs(config)

	if appConfigs.HasId("") {
		return fmt.Errorf("application config requires ID to be set: %+v", appConfigs)
	}

	appIds := appConfigs.Ids()
	if len(appIds) > 1 {
		return fmt.Errorf("more than one application config found: %+v", appConfigs)
	}

	var appConfig server.ApplicationConfig

	if len(appIds) == 1 {
		conf, err := appConfigs.GetApplication(appIds[0])
		if err != nil {
			return err
		}
		appConfig = conf
	} else {
		appConfig = server.ApplicationConfig{
			Id:          "app",
			Name:        "app",
			ContextRoot: "/",
			Type:        appType,
			AppElement:  "application",
		}
	}

	appConfig.Location = appPath
	if contextRoot != "" {
		appConfig.ContextRoot = contextRoot
	}
	if appConfig.Type == "" {
		appConfig.Type = appType
	}

	templatePath, err := b.getConfigTemplate("app.tmpl")
	if err != nil {
		return fmt.Errorf("unable to get app config template\n%w", err)
	}
	t, err := template.New("app.tmpl").ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("unable to create app template\n%w", err)
	}

	configOverridesPath := filepath.Join(serverPath, "configDropins", "overrides")
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

func (b Base) contributeUserFeatures(layer libcnb.Layer) error {
	if len(b.UserFeatureDescriptor.Features) <= 0 {
		b.Logger.Debug("No user features found; skipping...")
		return nil
	}

	runtimeLibsPath := filepath.Join(layer.Path, "wlp", "usr", "extension", "lib")
	runtimeFeaturesPath := filepath.Join(runtimeLibsPath, "features")
	if err := os.MkdirAll(runtimeFeaturesPath, 0755); err != nil {
		return err
	}

	if err := b.UserFeatureDescriptor.ResolveFeatures(); err != nil {
		return err
	}

	featureInstaller := NewFeatureInstaller(
		filepath.Join(layer.Path, "wlp"),
		b.ServerName,
		filepath.Join(b.BuildpackPath, "templates", "features.tmpl"),
		b.UserFeatureDescriptor.Features)

	if err := featureInstaller.Install(); err != nil {
		return err
	}
	if err := featureInstaller.Enable(); err != nil {
		return err
	}

	return nil
}

func (Base) Name() string {
	return "base"
}

func (b Base) getConfigTemplate(template string) (string, error) {
	// Check if the config template has been provided in /templates
	templatesPath := filepath.Join("/", "templates", template)
	exists, err := sherpa.FileExists(templatesPath)
	if err != nil {
		return "", fmt.Errorf("unable to check for template at %s\n%w", templatesPath, err)
	}
	if exists {
		return templatesPath, nil
	}

	// Check if the config template has been provided in a liberty binding
	// TODO: Remove this in next major release
	if bindingPath, ok := b.LibertyBinding.SecretFilePath(template); ok {
		b.Logger.Info(color.YellowString("Warning: Providing config templates via binding is deprecated. Mount the config templates to /templates instead."))
		return bindingPath, nil
	}

	// Return the default config template
	return filepath.Join(b.BuildpackPath, "templates", template), nil
}

func getTemplatesChecksum() (string, error) {
	templatesPath := filepath.Join("/", "templates")
	exists, err := sherpa.DirExists(templatesPath)
	if err != nil {
		return "", fmt.Errorf("unable to check if /templates mount provided\n%w", err)
	}
	if !exists {
		return "", nil
	}
	templatesSum, err := sherpa.NewFileListingHash(templatesPath)
	if err != nil {
		return "", fmt.Errorf("unable to get checksum for %s\n%w", templatesPath, err)
	}
	return templatesSum, nil
}
