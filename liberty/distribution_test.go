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
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/effect/mocks"
	"github.com/stretchr/testify/mock"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/liberty/liberty"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testDistribution(t *testing.T, when spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx           libcnb.BuildContext
		baseLayerPath string
	)

	it.Before(func() {
		var err error

		ctx.Layers.Path, err = ioutil.TempDir("", "home-layers")
		Expect(err).NotTo(HaveOccurred())
		baseLayerPath = filepath.Join(ctx.Layers.Path, "base")
		Expect(os.Mkdir(baseLayerPath, 0755)).To(Succeed())
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

		distro, be := liberty.NewDistribution(dep, dc, "defaultServer", ctx.Application.Path, baseLayerPath, []string{}, effect.NewExecutor())
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

	it("installs iFixes", func() {
		dep := libpak.BuildpackDependency{
			ID:     "open-liberty-runtime",
			URI:    "https://localhost/stub-liberty-runtime.zip",
			SHA256: "e71b55142699b277357d486eeb6244c71a0be3657a96a4286e30b27ceff34b17",
		}
		dc := libpak.DependencyCache{CachePath: "testdata"}

		executor := &mocks.Executor{}
		executor.On("Execute", mock.Anything).Return(nil)

		iFixesPath := filepath.Join(baseLayerPath, "conf", "ifixes")
		Expect(os.MkdirAll(iFixesPath, 0755)).To(Succeed())
		iFixPath := filepath.Join(iFixesPath, "210012-wlp-archive-ifph12345.jar")
		Expect(os.WriteFile(iFixPath, []byte{}, 0644)).To(Succeed())

		distro, _ := liberty.NewDistribution(dep, dc, "defaultServer", ctx.Application.Path, baseLayerPath, []string{}, executor)
		distro.Logger = bard.NewLogger(io.Discard)

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = distro.Contribute(layer)
		Expect(err).NotTo(HaveOccurred())

		installLibertyExecution := executor.Calls[0].Arguments[0].(effect.Execution)
		Expect(installLibertyExecution.Command).To(Equal(filepath.Join(layer.Path, "bin", "server")))
		Expect(installLibertyExecution.Args).To(Equal([]string{"create", "defaultServer"}))
		Expect(installLibertyExecution.Dir).To(Equal(layer.Path))

		installIFixExecution := executor.Calls[1].Arguments[0].(effect.Execution)
		Expect(installIFixExecution.Command).To(Equal("java"))
		Expect(installIFixExecution.Args).To(Equal([]string{"-jar", iFixPath, "--installLocation", layer.Path}))
	})

	it("installs features", func() {
		dep := libpak.BuildpackDependency{
			ID:     "open-liberty-runtime",
			URI:    "https://localhost/stub-liberty-runtime.zip",
			SHA256: "e71b55142699b277357d486eeb6244c71a0be3657a96a4286e30b27ceff34b17",
		}
		dc := libpak.DependencyCache{CachePath: "testdata"}

		executor := &mocks.Executor{}
		executor.On("Execute", mock.Anything).Return(nil)

		features := []string{"foo", "bar", "baz"}
		distro, _ := liberty.NewDistribution(dep, dc, "defaultServer", ctx.Application.Path, baseLayerPath, features, executor)
		distro.Logger = bard.NewLogger(io.Discard)

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = distro.Contribute(layer)
		Expect(err).NotTo(HaveOccurred())

		installLibertyExecution := executor.Calls[0].Arguments[0].(effect.Execution)
		Expect(installLibertyExecution.Command).To(Equal(filepath.Join(layer.Path, "bin", "server")))
		Expect(installLibertyExecution.Args).To(Equal([]string{"create", "defaultServer"}))
		Expect(installLibertyExecution.Dir).To(Equal(layer.Path))

		for i, call := range executor.Calls[1:] {
			installFeatureExecution := call.Arguments[0].(effect.Execution)
			Expect(installFeatureExecution.Command).To(Equal(filepath.Join(layer.Path, "bin", "featureUtility")))
			Expect(installFeatureExecution.Args).To(Equal([]string{"installFeature", features[i], "--acceptLicense"}))
		}
	})

}
