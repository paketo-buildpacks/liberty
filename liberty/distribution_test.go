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

package openliberty_test

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/liberty/liberty"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testDistribution(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx libcnb.BuildContext
	)

	it.Before(func() {
		var err error

		ctx.Layers.Path, err = ioutil.TempDir("", "home-layers")
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
	})

	it("contributes open liberty runtime", func() {
		dep := libpak.BuildpackDependency{
			ID:     "open-liberty-runtime",
			URI:    "https://localhost/stub-liberty-runtime.zip",
			SHA256: "e71b55142699b277357d486eeb6244c71a0be3657a96a4286e30b27ceff34b17",
		}
		dc := libpak.DependencyCache{CachePath: "testdata"}

		distro, be := openliberty.NewDistribution(dep, dc, "defaultPath", ctx.Application.Path)
		distro.Logger = bard.NewLogger(io.Discard)
		Expect(be.Name).To(Equal("open-liberty-runtime"))
		Expect(be.Launch).To(BeTrue())

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = distro.Contribute(layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(layer.Launch).To(BeTrue())
		Expect(filepath.Join(layer.Path, "bin", "server")).To(BeARegularFile())
		Expect(filepath.Join(layer.Path, "usr", "servers", "defaultServer", "apps")).To(BeADirectory())
		Expect(layer.LaunchEnvironment["BPI_LIBERTY_RUNTIME_ROOT.default"]).To(Equal(layer.Path))
	})
}
