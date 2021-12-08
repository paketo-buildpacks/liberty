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
	)

	it.Before(func() {
		var err error

		ctx.Application.Path, err = ioutil.TempDir("", "open-liberty-app")
		Expect(err).NotTo(HaveOccurred())

		ctx.Platform.Path, err = ioutil.TempDir("", "open-liberty-test-platform")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "META-INF"), 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "MANIFEST.MF"), []byte{}, 0644)).To(Succeed())
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
	})

	it("passes by default", func() {
		result, err := detect.Detect(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(libcnb.DetectResult{
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
		}))
	})

	it("passes if application.xml is present", func() {
		Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "application.xml"), []byte{}, 0644)).To(Succeed())

		result, err := detect.Detect(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(libcnb.DetectResult{
			Pass: true,
			Plans: []libcnb.BuildPlan{
				{
					Provides: []libcnb.BuildPlanProvide{
						{Name: openliberty.PlanEntryOpenLiberty},
						{Name: openliberty.PlanEntryJVMApplicationPackage},
					},

					Requires: []libcnb.BuildPlanRequire{
						{Name: openliberty.PlanEntryJRE, Metadata: map[string]interface{}{"launch": true}},
						{Name: openliberty.PlanEntryJVMApplicationPackage},
						{Name: openliberty.PlanEntryOpenLiberty},
					},
				},
			},
		}))
	})

	it("passes if a WEB-INF directory exists", func() {
		path := filepath.Join(ctx.Application.Path, "WEB-INF")
		Expect(os.MkdirAll(path, 0755)).To(Succeed())

		result, err := detect.Detect(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(libcnb.DetectResult{
			Pass: true,
			Plans: []libcnb.BuildPlan{
				{
					Provides: []libcnb.BuildPlanProvide{
						{Name: openliberty.PlanEntryOpenLiberty},
						{Name: openliberty.PlanEntryJVMApplicationPackage},
					},

					Requires: []libcnb.BuildPlanRequire{
						{Name: openliberty.PlanEntryJRE, Metadata: map[string]interface{}{"launch": true}},
						{Name: openliberty.PlanEntryJVMApplicationPackage},
						{Name: openliberty.PlanEntryOpenLiberty},
					},
				},
			},
		}))
	})

	it("passes with no manifest present", func() {
		Expect(os.Remove(filepath.Join(ctx.Application.Path, "META-INF", "MANIFEST.MF"))).To(Succeed())
		path := filepath.Join(ctx.Application.Path, "WEB-INF")
		Expect(os.MkdirAll(path, 0755)).To(Succeed())

		result, err := detect.Detect(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(libcnb.DetectResult{
			Pass: true,
			Plans: []libcnb.BuildPlan{
				{
					Provides: []libcnb.BuildPlanProvide{
						{Name: openliberty.PlanEntryOpenLiberty},
						{Name: openliberty.PlanEntryJVMApplicationPackage},
					},

					Requires: []libcnb.BuildPlanRequire{
						{Name: openliberty.PlanEntryJRE, Metadata: map[string]interface{}{"launch": true}},
						{Name: openliberty.PlanEntryJVMApplicationPackage},
						{Name: openliberty.PlanEntryOpenLiberty},
					},
				},
			},
		}))
	})

	it("fails if a Main-Class is present", func() {
		Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "MANIFEST.MF"), []byte("Main-Class: com.java.Helloworld"), 0644))

		result, err := detect.Detect(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(libcnb.DetectResult{Pass: false}))
	})
}
