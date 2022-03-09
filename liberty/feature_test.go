package liberty_test

import (
	"encoding/xml"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/liberty/liberty"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/sclevine/spec"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func testFeatures(t *testing.T, when spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		configRoot  string
		runtimeRoot string
	)

	it.Before(func() {
		var err error
		configRoot, err = ioutil.TempDir("", "config")
		Expect(err).NotTo(HaveOccurred())
		runtimeRoot, err = ioutil.TempDir("", "liberty-runtime")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.MkdirAll(filepath.Join(runtimeRoot, "usr", "servers", "defaultServer", "configDropins", "defaults"), 0755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(runtimeRoot, "usr", "extension", "lib", "features"), 0755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(runtimeRoot, "usr", "extension", "lib", "features"), 0755)).To(Succeed())
	})

	it.After(func() {
		Expect(os.RemoveAll(configRoot)).To(Succeed())
		Expect(os.RemoveAll(runtimeRoot)).To(Succeed())
	})

	when("feature descriptor is not provided", func() {
		it("should not load any features", func() {
			desc, err := liberty.ReadFeatureDescriptor(configRoot, bard.NewLogger(ioutil.Discard))
			Expect(err).NotTo(HaveOccurred())
			Expect(desc.Features).To(BeEmpty())
		})
	})

	when("reading a valid feature descriptor", func() {
		it.Before(func() {
			features := `[[features]]
                               name = "testFeature"
                               uri = "file:///test.feature_1.0.0.jar"
                               version = "1.0.0"
                               dependencies = ["test-1.0"]`
			Expect(os.WriteFile(filepath.Join(configRoot, "features.toml"), []byte(features), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(configRoot, "test.feature_1.0.0.jar"), []byte{}, 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(configRoot, "test.feature_1.0.0.mf"), []byte{}, 0644)).To(Succeed())
		})

		it("should resolve features", func() {
			desc, err := liberty.ReadFeatureDescriptor(configRoot, bard.NewLogger(ioutil.Discard))
			Expect(err).NotTo(HaveOccurred())
			Expect(desc.Features).To(HaveLen(1))

			Expect(desc.ResolveFeatures()).To(Succeed())

			feature := *desc.Features[0]
			Expect(feature.Name).To(Equal("testFeature"))
			Expect(feature.Version).To(Equal("1.0.0"))
			Expect(feature.ResolvedPath).To(Equal(filepath.Join(configRoot, "test.feature_1.0.0.jar")))
			Expect(feature.ManifestPath).To(Equal(filepath.Join(configRoot, "test.feature_1.0.0.mf")))
			Expect(feature.Dependencies).To(Equal([]string{"test-1.0"}))
		})
	})

	when("reading a feature descriptor with an bad feature URL", func() {
		it.Before(func() {
			features := `[[features]]
                               name = "testFeature"
                               uri = "file:///test.feature_1.0.0.jar"
                               version = "1.0.0"
                               dependencies = ["test-1.0"]`
			Expect(os.WriteFile(filepath.Join(configRoot, "features.toml"), []byte(features), 0644)).To(Succeed())
		})
		it("should throw an error", func() {
			desc, err := liberty.ReadFeatureDescriptor(configRoot, bard.NewLogger(ioutil.Discard))
			Expect(err).NotTo(HaveOccurred())
			Expect(desc.ResolveFeatures()).ToNot(Succeed())
		})
	})

	when("installing the features", func() {
		it.Before(func() {
			Expect(os.WriteFile(filepath.Join(configRoot, "features.tmpl"), []byte{}, 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(configRoot, "test.feature_1.0.0.jar"), []byte{}, 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(configRoot, "test.feature_1.0.0.mf"), []byte{}, 0644)).To(Succeed())
		})

		it("should link the feature jar to usr/extension/lib", func() {
			features := []*liberty.Feature{
				{
					Name:         "testFeature",
					URI:          "file:///test.feature_1.0.0.jar",
					Version:      "1.0.0",
					Dependencies: []string{"test-1.0"},
					ResolvedPath: filepath.Join(configRoot, "test.feature_1.0.0.jar"),
					ManifestPath: filepath.Join(configRoot, "test.feature_1.0.0.mf"),
				},
			}
			installer := liberty.NewFeatureInstaller(runtimeRoot, "defaultServer", filepath.Join(configRoot, "features.tmpl"), features)
			Expect(installer.Install()).To(Succeed())
			Expect(filepath.Join(runtimeRoot, "usr", "extension", "lib", "test.feature_1.0.0.jar")).To(BeARegularFile())
			Expect(filepath.Join(runtimeRoot, "usr", "extension", "lib", "features", "test.feature_1.0.0.mf")).To(BeARegularFile())
		})
	})

	when("enabling the features", func() {
		it.Before(func() {
			template := `<?xml version="1.0" encoding="UTF-8"?>
                         <server>
                           <!-- Enable user features -->
                           <featureManager>
                             {{ range $val := . }}
                                 <feature>{{ $val }}</feature>
                             {{ end }}
                           </featureManager>
                         </server>`
			Expect(os.WriteFile(filepath.Join(configRoot, "features.tmpl"), []byte(template), 0644)).To(Succeed())
		})

		it("should create the feature config", func() {
			features := []*liberty.Feature{
				{
					Name:         "testFeature",
					URI:          "file:///test.feature_1.0.0.jar",
					Version:      "1.0.0",
					Dependencies: []string{"test-1.0"},
					ResolvedPath: filepath.Join(configRoot, "test.feature_1.0.0.jar"),
					ManifestPath: filepath.Join(configRoot, "test.feature_1.0.0.mf"),
				},
			}
			installer := liberty.NewFeatureInstaller(runtimeRoot, "defaultServer", filepath.Join(configRoot, "features.tmpl"), features)
			featureConfigPath := filepath.Join(runtimeRoot, "usr", "servers", "defaultServer", "configDropins", "defaults", "features.xml")
			Expect(installer.Enable()).To(Succeed())
			Expect(featureConfigPath).To(BeARegularFile())

			xmlFile, err := os.Open(featureConfigPath)
			Expect(err).ToNot(HaveOccurred())
			defer xmlFile.Close()

			bytes, err := ioutil.ReadAll(xmlFile)
			Expect(err).ToNot(HaveOccurred())

			var featureConfig struct {
				XMLName        xml.Name `xml:"server"`
				FeatureManager struct {
					XMLName  xml.Name `xml:"featureManager"`
					Features []string `xml:"feature"`
				} `xml:"featureManager"`
			}

			err = xml.Unmarshal(bytes, &featureConfig)
			Expect(err).ToNot(HaveOccurred())

			featureList := featureConfig.FeatureManager.Features
			Expect(featureList).To(HaveLen(2))
			Expect(featureList).To(Equal([]string{"usr:testFeature", "test-1.0"}))
		})
	})
}
