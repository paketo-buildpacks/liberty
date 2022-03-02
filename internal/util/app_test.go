package util_test

import (
	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/sclevine/spec"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func testApp(t *testing.T, when spec.G, it spec.S) {
	var (
		Expect   = NewWithT(t).Expect
		testPath string
	)

	it.Before(func() {
		var err error
		testPath, err = ioutil.TempDir("", "app-utils")
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(testPath)).To(Succeed())
	})

	when("checking if an application is a JVM application package", func() {
		it("returns true if there META-INF/application.xml", func() {
			Expect(os.Mkdir(filepath.Join(testPath, "META-INF"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(testPath, "META-INF", "application.xml"), []byte{}, 0644)).To(Succeed())
			isJvmApp, err := util.IsJvmApplicationPackage(testPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(isJvmApp).To(BeTrue())
		})

		it("returns true if there WEB-INF/ directory", func() {
			Expect(os.MkdirAll(filepath.Join(testPath, "WEB-INF"), 0755)).To(Succeed())
			isJvmApp, err := util.IsJvmApplicationPackage(testPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(isJvmApp).To(BeTrue())
		})

		it("returns false if WEB-INF exists but is a file", func() {
			Expect(os.WriteFile(filepath.Join(testPath, "WEB-INF"), []byte{}, 0644)).To(Succeed())
			isJvmApp, err := util.IsJvmApplicationPackage(testPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(isJvmApp).To(BeFalse())
		})

		it("returns false if no WEB-INF or application.xml", func() {
			isJvmApp, err := util.IsJvmApplicationPackage(testPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(isJvmApp).To(BeFalse())
		})
	})

	when("checking for Main-Class in MANIFEST.MF", func() {
		it("returns true if Main-Class is defined in the META-INF/MANIFEST.MF", func() {
			Expect(os.Mkdir(filepath.Join(testPath, "META-INF"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(testPath, "META-INF", "MANIFEST.MF"), []byte("Main-Class: com.example.foo"), 0644)).To(Succeed())
			hasManifestDefined, err := util.ManifestHasMainClassDefined(testPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(hasManifestDefined).To(BeTrue())
		})

		it("returns false if Main-Class is not defined in the META-INF/MANIFEST.MF", func() {
			Expect(os.Mkdir(filepath.Join(testPath, "META-INF"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(testPath, "META-INF", "MANIFEST.MF"), []byte{}, 0644)).To(Succeed())
			hasManifestDefined, err := util.ManifestHasMainClassDefined(testPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(hasManifestDefined).To(BeFalse())
		})

		it("returns false if MANIFEST.MF does not exist", func() {
			hasManifestDefined, err := util.ManifestHasMainClassDefined(testPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(hasManifestDefined).To(BeFalse())
		})
	})
}
