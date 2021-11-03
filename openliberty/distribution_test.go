package openliberty_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/open-liberty/openliberty"
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

		distro, be := openliberty.NewDistribution(dep, dc, ctx.Application.Path)
		Expect(be.Name).To(Equal("open-liberty-runtime"))
		Expect(be.Launch).To(BeTrue())

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = distro.Contribute(layer)
		Expect(err).NotTo(HaveOccurred())

		Expect(layer.Launch).To(BeTrue())
		Expect(filepath.Join(layer.Path, "bin", "server")).To(BeARegularFile())
		Expect(filepath.Join(layer.Path, "usr", "servers", "defaultServer", "dropins")).To(BeADirectory())
		Expect(layer.LaunchEnvironment["BPI_OL_RUNTIME_ROOT.default"]).To(Equal(layer.Path))
	})
}
