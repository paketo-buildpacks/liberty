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

package helper_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/liberty/helper"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testLink(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect
		linker helper.FileLinker

		appDir       string
		configDir    string
		layerDir     string
		baseLayerDir string
	)

	it.Before(func() {
		var err error

		appDir, err = ioutil.TempDir("", "execd-helper-apps")
		Expect(err).NotTo(HaveOccurred())

		layerDir, err = ioutil.TempDir("", "execd-helper-layers")
		Expect(err).NotTo(HaveOccurred())

		baseLayerDir, err = ioutil.TempDir("", "base-layer")
		Expect(err).NotTo(HaveOccurred())

		configDir = filepath.Join(baseLayerDir, "conf")
		Expect(os.MkdirAll(configDir, 0755)).To(Succeed())
	})

	it.After(func() {
		Expect(os.RemoveAll(appDir)).To(Succeed())
		Expect(os.RemoveAll(layerDir)).To(Succeed())
		Expect(os.RemoveAll(baseLayerDir)).To(Succeed())
	})

	it("fails as BPI_OL_RUNTIME_ROOT is required", func() {
		Expect("/workspace").NotTo(BeADirectory())
		Expect("/layers/paketo-buildpacks_liberty/open-liberty-runtime").NotTo(BeADirectory())

		_, err := linker.Execute()
		Expect(err).To(MatchError("$BPI_OL_RUNTIME_ROOT must be set"))
	})

	context("with explicit env vars set to valid dirs", func() {
		it.Before(func() {
			Expect(os.Setenv("BPI_LIBERTY_DROPIN_DIR", appDir)).To(Succeed())
			Expect(os.Setenv("BPI_LIBERTY_RUNTIME_ROOT", layerDir)).To(Succeed())
			Expect(os.Setenv("BPI_LIBERTY_BASE_ROOT", baseLayerDir)).To(Succeed())
			Expect(os.Setenv("BP_LIBERTY_SERVER_NAME", "defaultServer")).To(Succeed())

			Expect(os.MkdirAll(filepath.Join(layerDir, "usr", "servers", "defaultServer", "apps"), 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(layerDir, "usr", "servers", "defaultServer", "configDropins", "overrides"), 0755)).To(Succeed())

			Expect(os.WriteFile(filepath.Join(layerDir, "usr", "servers", "defaultServer", "server.xml"), []byte("<server/>"), 0644)).To(Succeed())

			templatesDir := filepath.Join(baseLayerDir, "templates")
			Expect(os.MkdirAll(templatesDir, 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(templatesDir, "app.tmpl"), []byte{}, 0644)).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BPI_LIBERTY_DROPIN_DIR")).To(Succeed())
			Expect(os.Unsetenv("BPI_LIBERTY_RUNTIME_ROOT")).To(Succeed())
			Expect(os.Unsetenv("BPI_LIBERTY_BASE_ROOT")).To(Succeed())
			Expect(os.Unsetenv("BP_LIBERTY_SERVER_NAME")).To(Succeed())

			Expect(os.RemoveAll(filepath.Join(layerDir, "usr", "servers", "defaultServer", "apps"))).To(Succeed())
			Expect(os.RemoveAll(filepath.Join(layerDir, "usr", "servers", "defaultServer", "configDropins", "overrides"))).To(Succeed())
		})

		it("works", func() {
			_, err := linker.Execute()
			Expect(err).NotTo(HaveOccurred())

			resolvedAppDir, err := filepath.EvalSymlinks(appDir)
			Expect(err).NotTo(HaveOccurred())

			linkName := filepath.Join(layerDir, "usr", "servers", "defaultServer", "apps", "app")
			resolved, err := filepath.EvalSymlinks(linkName)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(Equal(resolvedAppDir))

			appConfigPath := filepath.Join(layerDir, "usr", "servers", "defaultServer", "configDropins", "overrides", "app.xml")
			Expect(appConfigPath).To(BeARegularFile())
		})
	})

	context("when building a packaged server containing a wlp directory", func() {
		it.Before(func() {
			Expect(os.Setenv("BPI_LIBERTY_DROPIN_DIR", appDir)).To(Succeed())
			Expect(os.Setenv("BPI_LIBERTY_RUNTIME_ROOT", layerDir)).To(Succeed())
			Expect(os.Setenv("BPI_LIBERTY_BASE_ROOT", baseLayerDir)).To(Succeed())

			Expect(os.MkdirAll(filepath.Join(layerDir, "usr", "servers", "defaultServer"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(layerDir, "usr", "servers", "defaultServer", "server.xml"), []byte("<server/>"), 0644)).To(Succeed())

			packagedServerDir := filepath.Join(appDir, "wlp", "usr", "servers", "defaultServer")
			Expect(os.MkdirAll(packagedServerDir, 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(packagedServerDir, "server.xml"), []byte{}, 0644)).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BPI_LIBERTY_DROPIN_DIR")).To(Succeed())
			Expect(os.Unsetenv("BPI_LIBERTY_RUNTIME_ROOT")).To(Succeed())
			Expect(os.Unsetenv("BPI_LIBERTY_BASE_ROOT")).To(Succeed())
			Expect(os.RemoveAll(filepath.Join(layerDir, "usr"))).To(Succeed())
			Expect(os.RemoveAll(filepath.Join(appDir, "wlp"))).To(Succeed())
		})

		it("replaces the runtime's user directory with app's wlp directory", func() {
			_, err := linker.Execute()
			Expect(err).NotTo(HaveOccurred())

			resolvedAppDir, err := filepath.EvalSymlinks(filepath.Join(appDir, "wlp", "usr"))
			Expect(err).NotTo(HaveOccurred())

			linkName := filepath.Join(layerDir, "usr")
			resolved, err := filepath.EvalSymlinks(linkName)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(Equal(resolvedAppDir))
		})
	})

	context("when building a packaged server containing a usr directory", func() {
		it.Before(func() {
			Expect(os.Setenv("BPI_LIBERTY_DROPIN_DIR", appDir)).To(Succeed())
			Expect(os.Setenv("BPI_LIBERTY_RUNTIME_ROOT", layerDir)).To(Succeed())
			Expect(os.Setenv("BPI_LIBERTY_BASE_ROOT", baseLayerDir)).To(Succeed())

			Expect(os.MkdirAll(filepath.Join(layerDir, "usr", "servers", "defaultServer"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(layerDir, "usr", "servers", "defaultServer", "server.xml"), []byte("<server/>"), 0644)).To(Succeed())

			packagedServerDir := filepath.Join(appDir, "usr", "servers", "defaultServer")
			Expect(os.MkdirAll(packagedServerDir, 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(packagedServerDir, "server.xml"), []byte{}, 0644)).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BPI_LIBERTY_DROPIN_DIR")).To(Succeed())
			Expect(os.Unsetenv("BPI_LIBERTY_RUNTIME_ROOT")).To(Succeed())
			Expect(os.Unsetenv("BPI_LIBERTY_BASE_ROOT")).To(Succeed())
			Expect(os.RemoveAll(filepath.Join(layerDir, "usr"))).To(Succeed())
		})

		it("replaces the runtime's user directory with app's usr directory", func() {
			_, err := linker.Execute()
			Expect(err).NotTo(HaveOccurred())

			resolvedAppDir, err := filepath.EvalSymlinks(filepath.Join(appDir, "usr"))
			Expect(err).NotTo(HaveOccurred())

			resolved, err := filepath.EvalSymlinks(filepath.Join(layerDir, "usr"))
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(Equal(resolvedAppDir))
		})
	})

	context("when contributing user features", func() {
		it.Before(func() {
			Expect(os.MkdirAll(filepath.Join(baseLayerDir, "usr", "servers", "defaultServer", "configDropins", "defaults"), 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(baseLayerDir, "usr", "extension", "lib", "features"), 0755)).To(Succeed())
			features := `[[features]]
                               name = "testFeature"
                               uri = "file:///test.feature_1.0.0.jar"
                               version = "1.0.0"
                               dependencies = ["test-1.0"]`
			Expect(os.WriteFile(filepath.Join(configDir, "features.toml"), []byte(features), 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(configDir, "test.feature_1.0.0.jar"), []byte{}, 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(configDir, "test.feature_1.0.0.mf"), []byte{}, 0644)).To(Succeed())
			template := `<?xml version="1.0" encoding="UTF-8"?>
                         <server>
                           <!-- Enable user features -->
                           <featureManager>
                             {{ range $val := . }}
                                 <feature>{{ $val }}</feature>
                             {{ end }}
                           </featureManager>
                         </server>`
			Expect(os.WriteFile(filepath.Join(configDir, "features.tmpl"), []byte(template), 0644)).To(Succeed())
		})

		it.After(func() {
			Expect(os.RemoveAll(filepath.Join(baseLayerDir, "usr", "servers", "defaultServer", "configDropins", "defaults"))).To(Succeed())
			Expect(os.RemoveAll(filepath.Join(baseLayerDir, "usr", "extension"))).To(Succeed())
		})

		it("installs the features and creates the feature config", func() {
			featureLinker := helper.FileLinker{
				BaseLayerPath:   baseLayerDir,
				RuntimeRootPath: layerDir,
			}
			err := featureLinker.ContributeUserFeatures("defaultServer", filepath.Join(configDir, "features.tmpl"))
			Expect(err).NotTo(HaveOccurred())
			Expect(filepath.Join(layerDir, "usr", "extension", "lib", "test.feature_1.0.0.jar")).To(BeARegularFile())
			Expect(filepath.Join(layerDir, "usr", "extension", "lib", "features", "test.feature_1.0.0.mf")).To(BeARegularFile())
			Expect(filepath.Join(layerDir, "usr", "servers", "defaultServer", "configDropins", "defaults", "features.xml")).To(BeARegularFile())
		})
	})
}
