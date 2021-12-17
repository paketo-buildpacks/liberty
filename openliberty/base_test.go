package openliberty_test

import (
	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/open-liberty/openliberty"
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
		srcTemplateDir := filepath.Join(ctx.Buildpack.Path, "templates")
		Expect(os.Mkdir(srcTemplateDir, 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(srcTemplateDir, "app.tmpl"), []byte{}, 0644)).To(Succeed())
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Buildpack.Path)).To(Succeed())
	})

	it("contributes configuration", func() {
		base := openliberty.NewBase(ctx.Buildpack.Path)
		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = base.Contribute(layer)
		Expect(err).ToNot(HaveOccurred())

		Expect(layer.Launch).To(BeTrue())
		Expect(filepath.Join(layer.Path, "templates")).To(BeADirectory())
		Expect(filepath.Join(layer.Path, "templates", "app.tmpl")).To(BeARegularFile())
		Expect(layer.LaunchEnvironment["BPI_OL_BASE_ROOT.default"]).To(Equal(layer.Path))
	})
}
