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

package util_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testArchive(t *testing.T, when spec.G, it spec.S) {
	var (
		Expect   = NewWithT(t).Expect
		testPath string
	)

	it.Before(func() {
		var err error
		testPath, err = os.MkdirTemp("", "archive-utils")
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
			contents, err := os.ReadFile(testFile)
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
			contents, err := os.ReadFile(testFile)
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
			contents, err := os.ReadFile(testFile)
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
			contents, err := os.ReadFile(testFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(contents).To(Equal([]byte("foo bar baz\n")))
		})
	})
}
