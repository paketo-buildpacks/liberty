package util_test

import (
	"github.com/paketo-buildpacks/open-liberty/internal/util"
	"github.com/sclevine/spec"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func testArchive(t *testing.T, when spec.G, it spec.S) {
	var (
		Expect   = NewWithT(t).Expect
		testPath string
	)

	it.Before(func() {
		var err error
		testPath, err = ioutil.TempDir("", "archive-utils")
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(testPath)).To(Succeed())
	})

	when("extracting a zip", func() {
		it("works", func() {
			zipPath := filepath.Join("testdata", "test.zip")
			zipFile, err := os.Open(zipPath)
			Expect(err).NotTo(HaveOccurred())
			defer zipFile.Close()
			Expect(util.Extract(zipFile, testPath, 0)).To(Succeed())

			testFile := filepath.Join(testPath, "test-dir", "test.txt")
			Expect(testFile).To(BeARegularFile())
			contents, err := ioutil.ReadFile(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(contents).To(Equal([]byte("foo bar baz\n")))
		})

		it("strips off the 1 directory", func() {
			zipPath := filepath.Join("testdata", "test.zip")
			zipFile, err := os.Open(zipPath)
			Expect(err).NotTo(HaveOccurred())
			defer zipFile.Close()
			Expect(util.Extract(zipFile, testPath, 1)).To(Succeed())

			testFile := filepath.Join(testPath, "test.txt")
			Expect(testFile).To(BeARegularFile())
			contents, err := ioutil.ReadFile(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(contents).To(Equal([]byte("foo bar baz\n")))
		})
	})

	when("extracting a tarball", func() {
		it("works with a .tar.gz extension", func() {
			zipPath := filepath.Join("testdata", "test.tar.gz")
			zipFile, err := os.Open(zipPath)
			Expect(err).NotTo(HaveOccurred())
			defer zipFile.Close()
			Expect(util.Extract(zipFile, testPath, 0)).To(Succeed())

			testFile := filepath.Join(testPath, "test-dir", "test.txt")
			Expect(testFile).To(BeARegularFile())
			contents, err := ioutil.ReadFile(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(contents).To(Equal([]byte("foo bar baz\n")))
		})

		it("strips off the 1 directory", func() {
			zipPath := filepath.Join("testdata", "test.tar.gz")
			zipFile, err := os.Open(zipPath)
			Expect(err).NotTo(HaveOccurred())
			defer zipFile.Close()
			Expect(util.Extract(zipFile, testPath, 1)).To(Succeed())

			testFile := filepath.Join(testPath, "test.txt")
			Expect(testFile).To(BeARegularFile())
			contents, err := ioutil.ReadFile(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(contents).To(Equal([]byte("foo bar baz\n")))
		})
	})
}
