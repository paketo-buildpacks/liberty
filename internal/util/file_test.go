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
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testFile(t *testing.T, when spec.G, it spec.S) {
	var (
		Expect   = NewWithT(t).Expect
		testPath string
	)

	it.Before(func() {
		var err error
		testPath, err = os.MkdirTemp("", "file-utils")
		Expect(err).NotTo(HaveOccurred())

		// EvalSymlinks on macOS resolves the temporary directory too so do that here or checking the symlinks will fail
		testPath, err = filepath.EvalSymlinks(testPath)
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(testPath)).To(Succeed())
	})

	when("linking a path", func() {
		it("should be successful if destination path does not exist", func() {
			srcPath := filepath.Join(testPath, "test-src")
			destPath := filepath.Join(testPath, "test-dest")
			Expect(os.WriteFile(srcPath, []byte{}, 0644)).To(Succeed())
			Expect(util.DeleteAndLinkPath(srcPath, destPath)).To(Succeed())
			Expect(destPath).To(BeARegularFile())
			resolved, err := filepath.EvalSymlinks(destPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(Equal(srcPath))
		})

		it("should remove destination file and successfully link new path", func() {
			srcPath := filepath.Join(testPath, "test-src")
			destPath := filepath.Join(testPath, "test-dest")
			Expect(os.WriteFile(srcPath, []byte{}, 0644)).To(Succeed())
			Expect(os.WriteFile(destPath, []byte{}, 0644)).To(Succeed())
			Expect(util.DeleteAndLinkPath(srcPath, destPath)).To(Succeed())
			Expect(destPath).To(BeARegularFile())
			resolved, err := filepath.EvalSymlinks(destPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(Equal(srcPath))
		})

		it("should remove destination directory and successfully link new path", func() {
			srcDir := filepath.Join(testPath, "test-src-dir")
			srcPath := filepath.Join(srcDir, "test-src")
			destDir := filepath.Join(testPath, "test-dest-dir")
			Expect(os.Mkdir(srcDir, 0755)).To(Succeed())
			Expect(os.Mkdir(destDir, 0755)).To(Succeed())
			Expect(os.WriteFile(srcPath, []byte{}, 0644)).To(Succeed())
			Expect(util.DeleteAndLinkPath(srcDir, destDir)).To(Succeed())
			Expect(filepath.Join(destDir, "test-src")).To(BeARegularFile())
			resolved, err := filepath.EvalSymlinks(destDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(Equal(srcDir))
		})

		it("should fail if source file does not exist", func() {
			srcPath := filepath.Join(testPath, "test-src")
			destPath := filepath.Join(testPath, "test-dest")
			err := util.DeleteAndLinkPath(srcPath, destPath)
			Expect(errors.Is(err, fs.ErrNotExist)).To(BeTrue())
			Expect(srcPath).ToNot(BeARegularFile())
			Expect(destPath).ToNot(BeARegularFile())
		})
	})

	when("getting files", func() {
		it("should return only xml files", func() {
			Expect(os.MkdirAll(filepath.Join(testPath, "foo"), 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(testPath, "bar", "baz"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(testPath, "foo", "foo.xml"), []byte{}, 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(testPath, "bar", "baz", "bar-baz.xml"), []byte{}, 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(testPath, "bar", "baz", "bar-baz.txt"), []byte{}, 0644)).To(Succeed())
			files, err := util.GetFiles(testPath, "*.xml")
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(Equal([]string{
				filepath.Join(testPath, "bar", "baz", "bar-baz.xml"),
				filepath.Join(testPath, "foo", "foo.xml"),
			}))
		})

		it("should return empty list of files for non-matching extension", func() {
			Expect(os.MkdirAll(filepath.Join(testPath, "foo"), 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(testPath, "bar", "baz"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(testPath, "foo", "foo.xml"), []byte{}, 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(testPath, "bar", "baz", "bar-baz.xml"), []byte{}, 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(testPath, "bar", "baz", "bar-baz.txt"), []byte{}, 0644)).To(Succeed())
			files, err := util.GetFiles(testPath, "*.dne")
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(BeEmpty())
		})

		it("should handle non-existent root", func() {
			files, err := util.GetFiles("dne", "*.dne")
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(BeEmpty())
		})
	})
}
