package openliberty_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/open-liberty/openliberty"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testDetect(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx    libcnb.DetectContext
		detect openliberty.Detect

		passingResult = libcnb.DetectResult{
			Pass: true,
			Plans: []libcnb.BuildPlan{
				{
					Provides: []libcnb.BuildPlanProvide{
						{Name: openliberty.PlanEntryOpenLiberty},
					},

					Requires: []libcnb.BuildPlanRequire{
						{Name: openliberty.PlanEntryJRE, Metadata: map[string]interface{}{"launch": true}},
						{Name: openliberty.PlanEntryJVMApplicationPackage},
						{Name: openliberty.PlanEntryOpenLiberty},
					},
				},
			},
		}
	)

	it.Before(func() {
		var err error

		ctx.Application.Path, err = ioutil.TempDir("", "openliberty")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "META-INF"), 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "MANIFEST.MF"), []byte{}, 0644)).To(Succeed())
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
	})

	it("fails if no files exist", func() {
		Expect(detect.Detect(ctx)).To(Equal(libcnb.DetectResult{Pass: false}))
	})

	it("passes if server.xml is present", func() {
		Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "server.xml"), []byte{}, 0644)).To(Succeed())
		ctx.Platform.Bindings = []libcnb.Binding{
			{
				Type: "open-liberty",
				Name: "open-liberty",
				Secret: map[string]string{
					"server.xml": "/path/to/server.xml",
				},
			},
		}

		result, err := detect.Detect(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(passingResult))
	})

	it("passes if a WEB-INF directory exists in the default location", func() {
		path := filepath.Join(ctx.Application.Path, "WEB-INF", "application.xml")

		Expect(os.MkdirAll(filepath.Dir(path), 0755)).To(Succeed())
		Expect(os.WriteFile(path, []byte{}, 0644)).To(Succeed())

		result, err := detect.Detect(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(passingResult))
	})

	context("directory exists in a custom location", func() {
		it.Before(func() {
			path := filepath.Join(ctx.Application.Path, "src", "main", "webapp", "WEB-INF", "application.xml")

			Expect(os.MkdirAll(filepath.Dir(path), 0755)).To(Succeed())
			Expect(os.WriteFile(path, []byte{}, 0644)).To(Succeed())

			Expect(os.Setenv("BP_OPENLIBERTY_WEBINF_PATH", "src/main/webapp"))
		})

		it("passes", func() {
			result, err := detect.Detect(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(passingResult))
		})

		it("fails if the user tries to escape the application root", func() {
			Expect(os.Setenv("BP_OPENLIBERTY_WEBINF_PATH", "..")).To(Succeed())

			result, err := detect.Detect(ctx)
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(libcnb.DetectResult{Pass: false}))
		})
	})

	it("passes with no manifest present", func() {
		Expect(os.Remove(filepath.Join(ctx.Application.Path, "META-INF", "MANIFEST.MF"))).To(Succeed())
		ctx.Platform.Bindings = []libcnb.Binding{
			{
				Type: "open-liberty",
				Name: "open-liberty",
				Secret: map[string]string{
					"server.xml": "/path/to/server.xml",
				},
			},
		}

		result, err := detect.Detect(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(passingResult))
	})

	it("fails if a Main-Class is present", func() {
		Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "server.xml"), []byte{}, 0644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "MANIFEST.MF"), []byte("Main-Class: com.java.Helloworld"), 0644))

		result, err := detect.Detect(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(libcnb.DetectResult{Pass: false}))
	})
}
