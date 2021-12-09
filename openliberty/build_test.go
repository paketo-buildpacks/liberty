package openliberty_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/buildpacks/libcnb"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/open-liberty/openliberty"
	"github.com/sclevine/spec"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx     libcnb.BuildContext
		builder openliberty.Build
	)

	it.Before(func() {
		var err error

		ctx.Application.Path, err = ioutil.TempDir("", "build-application")
		Expect(err).NotTo(HaveOccurred())

		ctx.Buildpack.Metadata = map[string]interface{}{
			"configurations": []map[string]interface{}{
				{"name": "BP_OPENLIBERTY_VERSION", "default": "21.0.11"},
				{"name": "BP_OPENLIBERTY_PROFILE", "default": "full"},
			},
			"dependencies": []map[string]interface{}{
				{"id": "open-liberty-runtime-full", "version": "21.0.11"},
				{"id": "open-liberty-runtime-microProfile4", "version": "21.0.10"},
			},
		}

		ctx.Layers.Path, err = ioutil.TempDir("", "build-layers")
		Expect(err).NotTo(HaveOccurred())
		builder = openliberty.Build{}
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
	})

	it("picks the latest full profile when no arguments are set", func() {
		buf := &bytes.Buffer{}
		builder.Logger = bard.NewLogger(buf)

		_, err := builder.Build(ctx)
		Expect(err).NotTo(HaveOccurred())

		output := buf.String()
		Expect(output).To(ContainSubstring("Choosing default version 21.0.11 for Open Liberty runtime"))
		Expect(output).To(ContainSubstring("Choosing default profile full for Open Liberty runtime"))
	})

	it("honors user set configuration values", func() {
		Expect(os.Setenv("BP_OPENLIBERTY_VERSION", "21.0.10")).To(Succeed())
		Expect(os.Setenv("BP_OPENLIBERTY_PROFILE", "microProfile4")).To(Succeed())

		buf := &bytes.Buffer{}
		builder.Logger = bard.NewLogger(buf)

		_, err := builder.Build(ctx)
		Expect(err).NotTo(HaveOccurred())

		output := buf.String()
		Expect(output).To(ContainSubstring("Choosing user-defined version 21.0.10 for Open Liberty runtime"))
		Expect(output).To(ContainSubstring("Choosing user-defined profile microProfile4 for Open Liberty runtime"))
	})
}
