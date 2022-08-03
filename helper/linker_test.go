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

package helper_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/liberty/helper"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testLink(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect
		linker helper.FileLinker

		appDir   string
		layerDir string
	)

	it.Before(func() {
		var err error

		appDir, err = ioutil.TempDir("", "execd-helper-apps")
		Expect(err).NotTo(HaveOccurred())
		appDir, err = filepath.EvalSymlinks(appDir)
		Expect(err).ToNot(HaveOccurred())

		layerDir, err = ioutil.TempDir("", "execd-helper-layers")
		Expect(err).NotTo(HaveOccurred())
		layerDir, err = filepath.EvalSymlinks(layerDir)
		Expect(err).ToNot(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(appDir)).To(Succeed())
		Expect(os.RemoveAll(layerDir)).To(Succeed())
	})

	it("fails as WLP_USER_DIR is required", func() {
		_, err := linker.Execute()
		Expect(err).To(MatchError("unable to configure\nunable to get server root path\n$WLP_USER_DIR must be set"))
	})

	context("with WLP_USER_DIR set", func() {
		it.Before(func() {
			Expect(os.Setenv("WLP_USER_DIR", layerDir)).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("WLP_USER_DIR")).To(Succeed())
		})

		it("still fails as BPI_LIBERTY_SERVER_NAME is required", func() {
			_, err := linker.Execute()
			Expect(err).To(MatchError("unable to configure\nunable to get server root path\n$BPI_LIBERTY_SERVER_NAME must be set"))
		})
	})

	context("with explicit env vars set to valid dirs", func() {
		it.Before(func() {
			Expect(os.Setenv("WLP_USER_DIR", layerDir)).To(Succeed())
			Expect(os.Setenv("BPI_LIBERTY_SERVER_NAME", "defaultServer")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("WLP_USER_DIR")).To(Succeed())
			Expect(os.Unsetenv("BPI_LIBERTY_SERVER_NAME")).To(Succeed())
		})

		it("works", func() {
			_, err := linker.Execute()
			Expect(err).NotTo(HaveOccurred())
		})
	})
}
