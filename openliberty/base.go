package openliberty

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/crush"
	"github.com/paketo-buildpacks/libpak/sherpa"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Base struct {
	BuildpackPath                   string
	LayerContributor                libpak.LayerContributor
	ConfigurationResolver           libpak.ConfigurationResolver
	DependencyCache                 libpak.DependencyCache
	ExternalConfigurationDependency *libpak.BuildpackDependency
	Logger                          bard.Logger
}

func NewBase(
	buildpackPath string,
	externalConfigurationDependency *libpak.BuildpackDependency,
	configurationResolver libpak.ConfigurationResolver,
	cache libpak.DependencyCache,
) Base {
	contributor := libpak.NewLayerContributor(
		"config",
		map[string]interface{}{},
		libcnb.LayerTypes{
			Launch: true,
		})

	b := Base{
		BuildpackPath:                   buildpackPath,
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
				return libcnb.Layer{}, fmt.Errorf("unable to contribute external configuration:\n%w", err)
			}
		}

		if err := b.ContributeUserFeatures(layer); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to contribute user features: %w", err)
		}

		b.Logger.Headerf("Contributing environment variables")
		layer.LaunchEnvironment.Default("BPI_OL_BASE_ROOT", layer.Path)

		return layer, nil
	})
}

func (b Base) ContributeExternalConfiguration(layer libcnb.Layer) error {
	b.Logger.Headerf("%s %s", b.ExternalConfigurationDependency.Name, b.ExternalConfigurationDependency.Version)

	artifact, err := b.DependencyCache.Artifact(*b.ExternalConfigurationDependency)
	if err != nil {
		return fmt.Errorf("unable to get dependency %v\n%w", b.ExternalConfigurationDependency.ID, err)
	}
	defer artifact.Close()

	confPath := filepath.Join(layer.Path, "conf")
	if err := os.MkdirAll(confPath, 0755); err != nil {
		return fmt.Errorf("unable to make external config directory:\n%w", err)
	}

	b.Logger.Bodyf("Expanding to %s", confPath)

	c := 0
	if s, ok := b.ConfigurationResolver.Resolve("BP_OPENLIBERTY_EXT_CONF_STRIP"); ok {
		if c, err = strconv.Atoi(s); err != nil {
			return fmt.Errorf("unable to parse %v to integer\n%w", s, err)
		}
	}

	if err := crush.ExtractTarGz(artifact, confPath, c); err != nil {
		return fmt.Errorf("unable to expand external configuration\n%w", err)
	}

	return nil
}

func (b Base) ContributeConfigTemplates(layer libcnb.Layer) error {
	b.Logger.Header("Contributing config templates")
	// Create config templates directory
	templateDir := filepath.Join(layer.Path, "templates")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return fmt.Errorf("unable to create config template directory '%v'\n%w", templateDir, err)
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
				return fmt.Errorf("unable to open source template file '%v'\n%w", srcPath, err)
			}
			defer in.Close()
			err = sherpa.CopyFile(in, destPath)
			if err != nil {
				return fmt.Errorf("unable to copy template from '%v' -> '%v'\n%w", srcPath, destPath, err)
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

func ReadFeatureDescriptor(baseRoot string, logger bard.Logger) (*FeatureDescriptor, error) {
	featuresTOML := filepath.Join(baseRoot, "features.toml")
	if _, err := os.Stat(featuresTOML); err != nil {
		logger.Debugf("No features descriptor found. Skipping.")
		return &FeatureDescriptor{}, nil
	}

	var featureDescriptor struct {
		Features []*Feature `toml:"features"`
	}

	if _, err := toml.DecodeFile(featuresTOML, &featureDescriptor); err != nil {
		return &FeatureDescriptor{}, fmt.Errorf("unable to decode features.toml:\n %w", err)
	}

	return &FeatureDescriptor{
		Path:     baseRoot,
		Features: featureDescriptor.Features,
		Logger:   logger,
	}, nil
}

func (d *FeatureDescriptor) ResolveFeatures() error {
	for i, feature := range d.Features {
		featureUrl, err := url.Parse(feature.URI)
		if err != nil {
			return fmt.Errorf("unable to parse URI for feature %v:\n%w", feature.Name, err)
		}
		if featureUrl.Scheme == "file" {
			if err := d.resolveFileFeature(d.Features[i]); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("unable to resolve feature %v: %v scheme unsupported", feature.Name, featureUrl.Scheme)
		}
	}
	return nil
}

func (d *FeatureDescriptor) resolveFileFeature(feature *Feature) error {
	featureUrl, err := url.Parse(feature.URI)
	if err != nil {
		return fmt.Errorf("unable to parse URI for feature %v:\n%w", feature.Name, err)
	}

	featurePath := featureUrl.Path[1:]   // Strip leading '/' required by file URLs
	ext := filepath.Ext(featurePath)[1:] // Strip '.' from extension
	var manifestPath string

	if ext != "jar" && ext != "esa" {
		return fmt.Errorf("unsupported feature packaging type for feature '%v': '%v'", feature.Name, ext)
	}

	if ext == "jar" {
		baseName := strings.TrimSuffix(featurePath, "."+ext)
		manifestPath = baseName + ".mf"
	}

	// Verify the necessary files are found
	resolvedFeaturePath := filepath.Join(d.Path, featurePath)
	if _, err := os.Stat(resolvedFeaturePath); err != nil {
		return fmt.Errorf("unable to find feature at '%v'", resolvedFeaturePath)
	}
	feature.ResolvedPath = resolvedFeaturePath

	if manifestPath == "" {
		return nil
	}

	resolvedManifestPath := filepath.Join(d.Path, manifestPath)
	if _, err := os.Stat(resolvedManifestPath); err != nil {
		return fmt.Errorf("unable to find manifest for feature '%v': %v", feature.Name, resolvedManifestPath)
	}
	feature.ManifestPath = resolvedManifestPath

	return nil
}
