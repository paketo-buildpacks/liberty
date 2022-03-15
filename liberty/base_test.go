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
 
package liberty_test

import (
	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/liberty/liberty"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/sclevine/spec"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func testBase(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx libcnb.BuildContext
	)

	it.Before(func() {
		var err error
		ctx.Layers.Path, err = ioutil.TempDir("", "base-layers")
		Expect(err).NotTo(HaveOccurred())

		ctx.Buildpack.Path, err = ioutil.TempDir("", "base-buildpack")

		ctx.Buildpack.Metadata = map[string]interface{}{
			"configurations": []map[string]interface{}{
				{"name": "BP_LIBERTY_SERVER_NAME", "default": "defaultServer"},
			},
		}
		srcTemplateDir := filepath.Join(ctx.Buildpack.Path, "templates")
		Expect(os.Mkdir(srcTemplateDir, 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(srcTemplateDir, "app.tmpl"), []byte{}, 0644)).To(Succeed())
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Buildpack.Path)).To(Succeed())
	})

	it("contributes configuration", func() {
		externalConfigurationDep := libpak.BuildpackDependency{
			ID:     "open-liberty-external-configuration",
			URI:    "https://localhost/stub-external-configuration-with-directory.tar.gz",
			SHA256: "060818cbcdc2008563f0f9e2428ecf4a199a5821c5b8b1dcd11a67666c1e2cd6",
			PURL:   "pkg:generic/ibm-open-libery-runtime-full@21.0.0.11?arch=amd64",
			CPEs:   []string{"cpe:2.3:a:ibm:liberty:21.0.0.11:*:*:*:*:*:*:*:*"},
		}
		dc := libpak.DependencyCache{CachePath: "testdata"}
		base := liberty.NewBase(ctx.Buildpack.Path, "defaultServer", &externalConfigurationDep, libpak.ConfigurationResolver{}, dc)
		base.Logger = bard.NewLogger(os.Stdout)
		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = base.Contribute(layer)
		Expect(err).ToNot(HaveOccurred())

		Expect(layer.Launch).To(BeTrue())
		Expect(filepath.Join(layer.Path, "templates")).To(BeADirectory())
		Expect(filepath.Join(layer.Path, "templates", "app.tmpl")).To(BeARegularFile())
		Expect(layer.LaunchEnvironment["BPI_LIBERTY_BASE_ROOT.default"]).To(Equal(layer.Path))
		Expect(filepath.Join(layer.Path, "conf", "external-configuration", "fixture-marker")).To(BeARegularFile())
	})
}
