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
	"github.com/paketo-buildpacks/liberty/internal/server"
	"github.com/paketo-buildpacks/libpak/sbom"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libjvm/count"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/crush"
	"github.com/paketo-buildpacks/libpak/effect"
)

type Distribution struct {
	Dependency            libpak.BuildpackDependency
	ApplicationPath       string
	InstallType           string
	ServerName            string
	Executor              effect.Executor
	DisableFeatureInstall bool
	Features              []string
	IFixes                []string
	LayerContributor      libpak.DependencyLayerContributor
	Logger                bard.Logger
}

func NewDistribution(
	dependency libpak.BuildpackDependency,
	cache libpak.DependencyCache,
	installType string,
	serverName string,
	applicationPath string,
	disableFeatureInstall bool,
	features []string,
	ifixes []string,
	executor effect.Executor,
) Distribution {
	contributor, _ := libpak.NewDependencyLayer(dependency, cache, libcnb.LayerTypes{
		Cache:  true,
		Launch: true,
	})

	contributor.ExpectedMetadata = map[string]interface{}{
		"dependency":  dependency,
		"server-name": serverName,
		"features":    features,
		"ifixes":      ifixes,
	}

	return Distribution{
		Dependency:            dependency,
		InstallType:           installType,
		ApplicationPath:       applicationPath,
		ServerName:            serverName,
		Executor:              executor,
		DisableFeatureInstall: disableFeatureInstall,
		Features:              features,
		IFixes:                ifixes,
		LayerContributor:      contributor,
	}
}

func (d Distribution) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	d.LayerContributor.Logger = d.Logger
	return d.LayerContributor.Contribute(layer, func(artifact *os.File) (libcnb.Layer, error) {
		d.Logger.Bodyf("Expanding to %s", layer.Path)
		if err := crush.ExtractZip(artifact, layer.Path, 1); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to expand Liberty Runtime\n%w", err)
		}

		if !d.DisableFeatureInstall {
			if err := server.InstallFeatures(layer.Path, d.ServerName, d.Executor, d.Logger); err != nil {
				return libcnb.Layer{}, fmt.Errorf("unable to install features to distribution\n%w", err)
			}
		} else {
			d.Logger.Debug("Skipping feature installation")
		}

		if err := server.InstallIFixes(layer.Path, d.IFixes, d.Executor, d.Logger); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to install iFixes to distribution\n%w", err)
		}

		// Create the output directory for Liberty
		outputDir := filepath.Join(layer.Path, "output")
		if err := createOutputDirectory(outputDir); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to create output directory\n%w", err)
		}
		layer.LaunchEnvironment.Override("WLP_OUTPUT_DIR", outputDir)

		libertyClasses, err := count.Classes(layer.Path)
		if err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to count liberty classes\n%w", err)
		}

		layer.LaunchEnvironment.Default("BPL_JVM_CLASS_ADJUSTMENT", strconv.Itoa(libertyClasses))

		// Used by exec.d helper
		layer.LaunchEnvironment.Default("BPI_LIBERTY_RUNTIME_ROOT", layer.Path)

		// set logging to write to the console. Using `server run` instead of `server start` ensures that
		// stdout/stderr are actually written to their respective streams instead of to `console.log`
		layer.LaunchEnvironment.Default("WLP_LOGGING_MESSAGE_SOURCE", "")
		layer.LaunchEnvironment.Default("WLP_LOGGING_CONSOLE_SOURCE", "message,trace,accessLog,ffdc,audit")

		// because of a liberty design decision, we can only force things to stdout if we are logging in
		// JSON format
		layer.LaunchEnvironment.Default("WLP_LOGGING_MESSAGE_FORMAT", "JSON")
		layer.LaunchEnvironment.Default("WLP_LOGGING_CONSOLE_FORMAT", "JSON")
		layer.LaunchEnvironment.Default("WLP_LOGGING_APPS_WRITE_JSON", "true")
		layer.LaunchEnvironment.Default("WLP_LOGGING_JSON_ACCESS_LOG_FIELDS", "default")

		if err := d.ContributeSBOM(layer); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to contribute SBOM\n%w", err)
		}

		return layer, nil
	})
}

func (d Distribution) ContributeSBOM(layer libcnb.Layer) error {
	sbomArtifact, err := d.Dependency.AsSyftArtifact()
	if err != nil {
		return fmt.Errorf("unable to get SBOM artifact %s\n%w", d.Dependency.ID, err)
	}
	artifacts := []sbom.SyftArtifact{sbomArtifact}

	installedIFixes, err := server.GetInstalledIFixes(layer.Path, d.Executor)
	if err != nil {
		return fmt.Errorf("unable to get installed iFixes\n%w", err)
	}
	for _, ifix := range installedIFixes {
		d.Logger.Debugf("Found installed iFix for APAR %s: %s\n", ifix.APAR, ifix.IFix)
		artifacts = append(artifacts, sbom.SyftArtifact{
			ID:        ifix.APAR,
			Name:      ifix.APAR,
			Version:   sbomArtifact.Version,
			Type:      "jar",
			Locations: []sbom.SyftLocation{{Path: ifix.IFix + ".jar"}},
		})
	}
	installedFeatures, err := server.GetInstalledFeatures(layer.Path, d.Executor)
	if err != nil {
		return fmt.Errorf("unable to get installed features\n%w", err)
	}
	var groupId string
	if d.InstallType == openLibertyInstall {
		groupId = "io.openliberty.features"
	} else {
		groupId = "com.ibm.websphere.appserver.features"
	}

	var version string
	if parts := strings.Split(sbomArtifact.PURL, "@"); len(parts) == 2 {
		version = parts[1]
	}
	d.Logger.Debugf("Adding features to SBOM: %s", strings.Join(installedFeatures, ", "))
	for _, feature := range installedFeatures {
		artifacts = append(artifacts, sbom.SyftArtifact{
			ID:      feature,
			Name:    feature,
			Version: sbomArtifact.Version,
			Type:    "esa",
			PURL:    fmt.Sprintf("pkg:maven/%s/%s@%s", groupId, feature, version),
		})
	}

	sbomPath := layer.SBOMPath(libcnb.SyftJSON)
	dep := sbom.NewSyftDependency(layer.Path, artifacts)

	d.Logger.Debugf("Writing Syft SBOM at %s: %+v", sbomPath, dep)
	if err := dep.WriteTo(sbomPath); err != nil {
		return fmt.Errorf("unable to write SBOM\n%w", err)
	}

	return nil
}

func (d Distribution) Name() string {
	return d.LayerContributor.LayerName()
}

func createOutputDirectory(path string) error {
	if err := os.Mkdir(path, 0774); err != nil {
		return fmt.Errorf("unable to create Liberty output directory\n%w", err)
	}
	if err := os.Chmod(path, 0774); err != nil {
		return fmt.Errorf("unable to chown Liberty output directory\n%w", err)
	}
	return nil
}
