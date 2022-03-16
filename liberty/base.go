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
	"fmt"
	"github.com/buildpacks/libcnb"
	"github.com/heroku/color"
	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/sherpa"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Base struct {
	BuildpackPath                   string
	ServerName                      string
	LayerContributor                libpak.LayerContributor
	ConfigurationResolver           libpak.ConfigurationResolver
	DependencyCache                 libpak.DependencyCache
	ExternalConfigurationDependency *libpak.BuildpackDependency
	Logger                          bard.Logger
}

func NewBase(
	buildpackPath string,
	serverName string,
	externalConfigurationDependency *libpak.BuildpackDependency,
	configurationResolver libpak.ConfigurationResolver,
	cache libpak.DependencyCache,
) Base {
	contributor := libpak.NewLayerContributor(
		"Open Liberty Config",
		map[string]interface{}{},
		libcnb.LayerTypes{
			Launch: true,
		})

	b := Base{
		BuildpackPath:                   buildpackPath,
		ServerName:                      serverName,
		LayerContributor:                contributor,
		ConfigurationResolver:           configurationResolver,
		DependencyCache:                 cache,
		ExternalConfigurationDependency: externalConfigurationDependency,
	}

	return b
}

func (b Base) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	b.LayerContributor.Logger = b.Logger

	return b.LayerContributor.Contribute(layer, func() (libcnb.Layer, error) {
		if err := b.ContributeConfigTemplates(layer); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to contribute config templates\n%w", err)
		}

		if b.ExternalConfigurationDependency != nil {
			if err := b.ContributeExternalConfiguration(layer); err != nil {
				return libcnb.Layer{}, fmt.Errorf("unable to contribute external configuration\n%w", err)
			}
		}

		if err := b.ContributeUserFeatures(layer); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to contribute user features\n%w", err)
		}

		listoffeatures, _ := b.ConfigurationResolver.Resolve("BP_LIBERTY_FEATURES")
		features := strings.Fields(listoffeatures)

		profile, _ := b.ConfigurationResolver.Resolve("BP_LIBERTY_PROFILE")
		installationLocation := fmt.Sprintf("/layers/paketo-buildpacks_liberty/open-liberty-runtime-%s", profile)

		for _, feature := range features {
			b.Logger.Info("Installing ", feature)

			executor := effect.NewExecutor()
			if err := executor.Execute(effect.Execution{
				Command: filepath.Join(installationLocation, "bin", "featureUtility"),
				Args:    []string{"installFeature", feature, "--acceptLicense"},
				Dir:     layer.Path,
				Stdout:  bard.NewWriter(b.Logger.InfoWriter(), bard.WithIndent(3)),
				Stderr:  bard.NewWriter(b.Logger.InfoWriter(), bard.WithIndent(3)),
			}); err != nil {
				return libcnb.Layer{}, fmt.Errorf("unable to apply ifix\n%w", err)
			}
		}

		layer.LaunchEnvironment.Default("BPI_LIBERTY_BASE_ROOT", layer.Path)
		layer.LaunchEnvironment.Default("BPI_LIBERTY_SERVER_NAME", b.ServerName)

		return layer, nil
	})
}

func (b Base) ContributeExternalConfiguration(layer libcnb.Layer) error {
	b.Logger.Headerf(color.BlueString("%s %s", b.ExternalConfigurationDependency.Name, b.ExternalConfigurationDependency.Version))

	artifact, err := b.DependencyCache.Artifact(*b.ExternalConfigurationDependency)
	if err != nil {
		return fmt.Errorf("unable to get dependency %s\n%w", b.ExternalConfigurationDependency.ID, err)
	}
	defer artifact.Close()

	confPath := filepath.Join(layer.Path, "conf")
	if err := os.MkdirAll(confPath, 0755); err != nil {
		return fmt.Errorf("unable to make external config directory\n%w", err)
	}

	b.Logger.Bodyf("Expanding to %s", confPath)

	c := 0
	if s, ok := b.ConfigurationResolver.Resolve("BP_LIBERTY_EXT_CONF_STRIP"); ok {
		if c, err = strconv.Atoi(s); err != nil {
			return fmt.Errorf("unable to parse %s to integer\n%w", s, err)
		}
	}

	if err := util.Extract(artifact, confPath, c); err != nil {
		return fmt.Errorf("unable to expand external configuration\n%w", err)
	}

	iFixesPath := filepath.Join(confPath, "ifixes")
	iFixesProvided, err := util.DirExists(iFixesPath)
	if err != nil {
		return fmt.Errorf("unable to check if iFixes were provided\n%w", err)
	}
	if iFixesProvided {
		profile, _ := b.ConfigurationResolver.Resolve("BP_LIBERTY_PROFILE")
		installLocation := fmt.Sprintf("/layers/paketo-buildpacks_liberty/open-liberty-runtime-%s", profile)
		return b.ApplyIFixes(iFixesPath, installLocation)
	}

	return nil
}

func (b Base) ApplyIFixes(iFixesPath string, installLocation string) error {
	iFixes, err := ioutil.ReadDir(iFixesPath)
	if err != nil {
		return fmt.Errorf("unable to read iFixes dir\n%w", err)
	}
	for _, iFix := range iFixes {
		b.Logger.Info("Installing ", iFix.Name())
		iFixPath := filepath.Join(iFixesPath, iFix.Name())
		executor := effect.NewExecutor()
		if err := executor.Execute(effect.Execution{
			Command: filepath.Join("java"),
			Args:    []string{"-jar", iFixPath, "--installLocation", installLocation},
			Stdout:  bard.NewWriter(b.Logger.InfoWriter(), bard.WithIndent(3)),
			Stderr:  bard.NewWriter(b.Logger.InfoWriter(), bard.WithIndent(3)),
		}); err != nil {
			return fmt.Errorf("unable to apply iFix\n%w", err)
		}
	}
	return nil
}

func (b Base) ContributeConfigTemplates(layer libcnb.Layer) error {
	// Create config templates directory
	templateDir := filepath.Join(layer.Path, "templates")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return fmt.Errorf("unable to create config template directory '%s'\n%w", templateDir, err)
	}

	srcDir := filepath.Join(b.BuildpackPath, "templates")
	entries, err := ioutil.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("unable to read source directory\n%w", err)
	}

	for _, entry := range entries {
		err := func() error {
			srcPath := filepath.Join(srcDir, entry.Name())
			b.Logger.Bodyf("Copying %v", srcPath)
			destPath := filepath.Join(templateDir, entry.Name())
			in, err := os.Open(srcPath)
			if err != nil {
				return fmt.Errorf("unable to open source template file '%s'\n%w", srcPath, err)
			}
			defer in.Close()
			err = sherpa.CopyFile(in, destPath)
			if err != nil {
				return fmt.Errorf("unable to copy template from '%s' -> '%s'\n%w", srcPath, destPath, err)
			}
			return nil
		}()

		if err != nil {
			return err
		}
	}
	return nil
}

func (b Base) ContributeUserFeatures(layer libcnb.Layer) error {
	featureDescriptor, err := ReadFeatureDescriptor(filepath.Join(layer.Path, "conf"), b.Logger)
	if err != nil {
		return err
	}

	if len(featureDescriptor.Features) <= 0 {
		b.Logger.Debug("No features found; skipping...")
		return nil
	}

	if err = featureDescriptor.ResolveFeatures(); err != nil {
		return err
	}

	return nil
}

func (Base) Name() string {
	return "base"
}
