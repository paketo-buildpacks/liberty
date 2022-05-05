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
	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/sclevine/spec"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func testFile(t *testing.T, when spec.G, it spec.S) {
	var (
		Expect   = NewWithT(t).Expect
		testPath string
	)

	it.Before(func() {
		var err error
		testPath, err = ioutil.TempDir("", "file-utils")
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

	when("checking a file exists", func() {
		it("should return true if path is a file", func() {
			path := filepath.Join(testPath, "test-file")
			Expect(os.WriteFile(path, []byte{}, 0644)).To(Succeed())
			exists, err := util.FileExists(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		it("should return true if path is a directory", func() {
			path := filepath.Join(testPath, "test-dir")
			Expect(os.Mkdir(path, 0755)).To(Succeed())
			exists, err := util.FileExists(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		it("should return false if path does not exist", func() {
			exists, err := util.FileExists(filepath.Join(testPath, "does-not-exist"))
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})

	when("checking a directory exists", func() {
		it("should return true if path is a directory", func() {
			path := filepath.Join(testPath, "test-dir")
			Expect(os.Mkdir(path, 0755)).To(Succeed())
			exists, err := util.DirExists(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		it("should return false if path is a file", func() {
			path := filepath.Join(testPath, "test-file")
			Expect(os.WriteFile(path, []byte{}, 0644)).To(Succeed())
			exists, err := util.DirExists(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		it("should return false if path does not exist", func() {
			exists, err := util.FileExists(filepath.Join(testPath, "does-not-exist"))
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})

	when("copying a directory", func() {
		it("works", func() {
			srcDir := filepath.Join(testPath, "src-dir")
			Expect(os.MkdirAll(filepath.Join(srcDir, "one", "two"), 0755)).To(Succeed())
			testFiles := []string{
				"file-root",
				filepath.Join("one", "file-one"),
				filepath.Join("one", "two", "file-two"),
			}
			for _, file := range testFiles {
				Expect(os.WriteFile(filepath.Join(srcDir, file), []byte(file), 0644)).To(Succeed())
			}
			destDir := filepath.Join(testPath, "dest-dir")
			Expect(os.Mkdir(destDir, 0755)).To(Succeed())
			Expect(util.CopyDir(srcDir, destDir)).To(Succeed())
			for _, file := range testFiles {
				destFile := filepath.Join(destDir, file)
				Expect(destFile).To(BeARegularFile())
				contents, err := ioutil.ReadFile(destFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(contents).To(Equal([]byte(file)))
			}
		})
	})

	when("copying a file", func() {
		it("works", func() {
			srcFile := filepath.Join(testPath, "src-file")
			payload := []byte("foo bar baz")
			Expect(os.WriteFile(srcFile, payload, 0644)).To(Succeed())
			destFile := filepath.Join(testPath, "dest-file")
			Expect(util.CopyFile(srcFile, destFile)).To(Succeed())
			Expect(destFile).To(BeARegularFile())
			contents, err := ioutil.ReadFile(destFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(contents).To(Equal(payload))
		})
	})
}
