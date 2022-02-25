package openliberty

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/open-liberty/internal/util"
)

type Feature struct {
	Name         string   `toml:"name"`
	Version      string   `toml:"version"`
	URI          string   `toml:"uri"`
	Dependencies []string `toml:"dependencies"`
	ResolvedPath string   `toml:"-"`
	ManifestPath string   `toml:"-"`
}

type FeatureDescriptor struct {
	Path     string
	Features []*Feature
	Logger   bard.Logger
}

func ReadFeatureDescriptor(configRoot string, logger bard.Logger) (*FeatureDescriptor, error) {
	featuresTOML := filepath.Join(configRoot, "features.toml")
	if _, err := os.Stat(featuresTOML); err != nil {
		logger.Debugf("No features descriptor found. Skipping.")
		return &FeatureDescriptor{}, nil
	}

	var featureDescriptor struct {
		Features []*Feature `toml:"features"`
	}

	if _, err := toml.DecodeFile(featuresTOML, &featureDescriptor); err != nil {
		return &FeatureDescriptor{}, fmt.Errorf("unable to decode features.toml\n%w", err)
	}

	return &FeatureDescriptor{
		Path:     configRoot,
		Features: featureDescriptor.Features,
		Logger:   logger,
	}, nil
}

func (d *FeatureDescriptor) ResolveFeatures() error {
	for i, feature := range d.Features {
		featureUrl, err := url.Parse(feature.URI)
		if err != nil {
			return fmt.Errorf("unable to parse URI for feature %s\n%w", feature.Name, err)
		}
		if featureUrl.Scheme == "file" {
			if err := d.resolveFileFeature(d.Features[i]); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("unable to resolve feature '%s': %s scheme unsupported", feature.Name, featureUrl.Scheme)
		}
	}
	return nil
}

func (d *FeatureDescriptor) resolveFileFeature(feature *Feature) error {
	featureUrl, err := url.Parse(feature.URI)
	if err != nil {
		return fmt.Errorf("unable to parse URI for feature %s\n%w", feature.Name, err)
	}

	featurePath := featureUrl.Path[1:]   // Strip leading '/' required by file URLs
	ext := filepath.Ext(featurePath)[1:] // Strip '.' from extension
	var manifestPath string

	if ext != "jar" && ext != "esa" {
		return fmt.Errorf("unsupported feature packaging type for feature '%s': '%s'", feature.Name, ext)
	}

	if ext == "jar" {
		baseName := strings.TrimSuffix(featurePath, "."+ext)
		manifestPath = baseName + ".mf"
	}

	// Verify the necessary files are found
	resolvedFeaturePath := filepath.Join(d.Path, featurePath)
	if _, err := os.Stat(resolvedFeaturePath); err != nil {
		return fmt.Errorf("unable to find feature at '%s'", resolvedFeaturePath)
	}
	feature.ResolvedPath = resolvedFeaturePath

	if manifestPath == "" {
		return nil
	}

	resolvedManifestPath := filepath.Join(d.Path, manifestPath)
	if _, err := os.Stat(resolvedManifestPath); err != nil {
		return fmt.Errorf("unable to find manifest for feature '%s': %s", feature.Name, resolvedManifestPath)
	}
	feature.ManifestPath = resolvedManifestPath

	return nil
}

type FeatureInstaller struct {
	RuntimeRootPath string
	ServerName      string
	TemplatePath    string
	Features        []*Feature
}

func NewFeatureInstaller(runtimeRootPath, serverName, templatePath string, features []*Feature) FeatureInstaller {
	return FeatureInstaller{
		RuntimeRootPath: runtimeRootPath,
		ServerName:      serverName,
		TemplatePath:    templatePath,
		Features:        features,
	}
}

// Install the user features into the Liberty server.
func (i FeatureInstaller) Install() error {
	for _, feature := range i.Features {
		if strings.HasSuffix(feature.ResolvedPath, ".jar") {
			if err := i.installJar(*feature); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("unable to install feature '%s' at '%s'", feature.Name, feature.ResolvedPath)
		}
	}
	return nil
}

func (i FeatureInstaller) installJar(feature Feature) error {
	runtimeLibsPath := filepath.Join(i.RuntimeRootPath, "usr", "extension", "lib")
	featureBase := filepath.Base(feature.ResolvedPath)
	if err := util.DeleteAndLinkPath(feature.ResolvedPath, filepath.Join(runtimeLibsPath, featureBase)); err != nil {
		return fmt.Errorf("unable to link feature '%s'\n%w", feature.Name, err)
	}

	if feature.ManifestPath == "" {
		return nil
	}

	manifestBase := filepath.Base(feature.ManifestPath)
	if err := util.DeleteAndLinkPath(feature.ManifestPath, filepath.Join(filepath.Join(runtimeLibsPath, "features", manifestBase))); err != nil {
		return fmt.Errorf("unable to link feature manifest for '%s'\n%w", feature.Name, err)
	}

	return nil
}

// Enable the user features.
func (i FeatureInstaller) Enable() error {
	var featuresToEnable []string
	for _, feature := range i.Features {
		featuresToEnable = append(featuresToEnable, "usr:"+feature.Name)
		if len(feature.Dependencies) > 0 {
			featuresToEnable = append(featuresToEnable, feature.Dependencies...)
		}
	}

	t, err := template.New("features.tmpl").ParseFiles(i.TemplatePath)
	if err != nil {
		return fmt.Errorf("unable to create features template\n%w", err)
	}

	configDefaultsPath := filepath.Join(i.RuntimeRootPath, "usr", "servers", i.ServerName, "configDropins", "defaults")
	if err := os.MkdirAll(configDefaultsPath, 0755); err != nil {
		return fmt.Errorf("unable to make config defaults directory\n%w", err)
	}

	featuresConfigPath := filepath.Join(configDefaultsPath, "features.xml")
	file, err := os.Create(featuresConfigPath)
	if err != nil {
		return fmt.Errorf("unable to create file '%s'\n%w", featuresConfigPath, err)
	}
	defer file.Close()
	err = t.Execute(file, featuresToEnable)
	if err != nil {
		return fmt.Errorf("unable to execute template\n%w", err)
	}

	return nil
}
