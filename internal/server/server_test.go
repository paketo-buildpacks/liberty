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

package server_test

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/liberty/internal/server"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"github.com/paketo-buildpacks/libpak/effect/mocks"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/mock"

	. "github.com/onsi/gomega"
)

func testServer(t *testing.T, when spec.G, it spec.S) {
	var (
		Expect   = NewWithT(t).Expect
		testPath string
		wlpPath  string
	)

	it.Before(func() {
		var err error
		testPath, err = ioutil.TempDir("", "server")
		Expect(err).NotTo(HaveOccurred())

		// EvalSymlinks on macOS resolves the temporary directory too so do that here or checking the symlinks will fail
		testPath, err = filepath.EvalSymlinks(testPath)
		Expect(err).NotTo(HaveOccurred())

		wlpPath = filepath.Join(testPath, "wlp")
		Expect(os.MkdirAll(filepath.Join(wlpPath, "usr", "servers", "defaultServer", "apps"), 0755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(wlpPath, "usr", "servers", "defaultServer", "dropins"), 0755)).To(Succeed())
	})

	it.After(func() {
		Expect(os.RemoveAll(testPath)).To(Succeed())
		Expect(os.RemoveAll(filepath.Join(wlpPath, "usr", "servers", "defaultServer", "apps"))).To(Succeed())
		Expect(os.RemoveAll(filepath.Join(wlpPath, "usr", "servers", "defaultServer", "dropins"))).To(Succeed())
	})

	when("changing the server user directory", func() {
		it("works with no configDropins in original user directory", func() {
			// Create new user directory
			newUserDir := filepath.Join(testPath, "new-user-dir")
			newServerDir := filepath.Join(newUserDir, "servers", "defaultServer")
			Expect(os.MkdirAll(newServerDir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(newServerDir, "server.xml"), []byte{}, 0644)).To(Succeed())

			Expect(server.SetUserDirectory(newUserDir, wlpPath, "defaultServer")).To(Succeed())
			newConfigPath, err := filepath.EvalSymlinks(server.GetServerConfigPath(newServerDir))
			Expect(err).NotTo(HaveOccurred())
			Expect(newConfigPath).To(Equal(filepath.Join(newServerDir, "server.xml")))

			Expect(os.RemoveAll(newUserDir)).To(Succeed())
		})

		it("copies over configDropins from original user directory", func() {
			// Create new user directory
			newUserDir := filepath.Join(testPath, "new-user-dir")
			newServerDir := filepath.Join(newUserDir, "servers", "defaultServer")
			Expect(os.MkdirAll(newServerDir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(newServerDir, "server.xml"), []byte{}, 0644)).To(Succeed())

			// Create configDropins in original directory
			configDropinsDir := filepath.Join(wlpPath, "servers", "defaultServer", "configDropins", "overrides")
			Expect(os.MkdirAll(configDropinsDir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(configDropinsDir, "test-config.xml"), []byte{}, 0644)).To(Succeed())

			Expect(server.SetUserDirectory(wlpPath, newUserDir, "defaultServer")).To(Succeed())
			Expect(filepath.Join(configDropinsDir, "test-config.xml")).To(BeARegularFile())

			Expect(os.RemoveAll(wlpPath)).To(Succeed())
			Expect(os.RemoveAll(newUserDir)).To(Succeed())
		})
	})

	when("checking if a server has installed apps", func() {
		it("finds war in apps directory", func() {
			serverPath := filepath.Join(wlpPath, "servers", "defaultServer")
			appPath := filepath.Join(serverPath, "apps", "test.war")
			Expect(os.MkdirAll(appPath, 0755)).To(Succeed())
			Expect(server.HasInstalledApps(serverPath)).To(BeTrue())
			Expect(os.RemoveAll(appPath)).To(Succeed())
		})

		it("finds ear in apps directory", func() {
			serverPath := filepath.Join(wlpPath, "servers", "defaultServer")
			appPath := filepath.Join(serverPath, "apps", "test.ear")
			Expect(os.MkdirAll(appPath, 0755)).To(Succeed())
			Expect(server.HasInstalledApps(serverPath)).To(BeTrue())
			Expect(os.RemoveAll(appPath)).To(Succeed())
		})

		it("finds war in dropins directory", func() {
			serverPath := filepath.Join(wlpPath, "servers", "defaultServer")
			appPath := filepath.Join(serverPath, "dropins", "test.war")
			Expect(os.MkdirAll(appPath, 0755)).To(Succeed())
			Expect(server.HasInstalledApps(serverPath)).To(BeTrue())
			Expect(os.RemoveAll(appPath)).To(Succeed())
		})

		it("finds ear in dropins directory", func() {
			serverPath := filepath.Join(wlpPath, "servers", "defaultServer")
			appPath := filepath.Join(serverPath, "dropins", "test.ear")
			Expect(os.MkdirAll(appPath, 0755)).To(Succeed())
			Expect(server.HasInstalledApps(serverPath)).To(BeTrue())
			Expect(os.RemoveAll(appPath)).To(Succeed())
		})
	})

	when("checking server list", func() {
		it("returns empty list when there are no servers", func() {
			emptyServerDir := filepath.Join(testPath, "servers")
			Expect(os.MkdirAll(emptyServerDir, 0755)).To(Succeed())
			serverList, err := server.GetServerList(testPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(serverList).To(BeEmpty())
		})

		it("lists servers correctly", func() {
			serversDir := filepath.Join(testPath, "servers")
			Expect(os.MkdirAll(filepath.Join(serversDir, "defaultServer"), 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(serversDir, "fooServer"), 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(serversDir, "testServer"), 0755)).To(Succeed())
			serverList, err := server.GetServerList(testPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(serverList).To(Equal([]string{"defaultServer", "fooServer", "testServer"}))
		})
	})

	when("installing iFixes", func() {
		var (
			iFixesPath string
			executor   = &mocks.Executor{}
		)

		it.Before(func() {
			iFixesPath = filepath.Join(testPath, "ifixes")
			Expect(os.MkdirAll(iFixesPath, 0755)).To(Succeed())
			executor.On("Execute", mock.Anything).Return(nil)
		})

		it.After(func() {
			Expect(os.RemoveAll(iFixesPath)).To(Succeed())
		})

		it("does not install anything if there are no iFixes", func() {
			Expect(server.InstallIFixes(wlpPath, []string{}, executor, bard.NewLogger(io.Discard))).To(Succeed())
			Expect(executor.Calls).To(BeEmpty())
		})

		it("works", func() {
			ifixes := []string{
				filepath.Join(iFixesPath, "210012-wlp-archive-ifph42489.jar"),
				filepath.Join(iFixesPath, "210012-wlp-archive-ifph12345.jar"),
			}
			Expect(os.WriteFile(ifixes[0], []byte{}, 0644)).To(Succeed())
			Expect(os.WriteFile(ifixes[1], []byte{}, 0644)).To(Succeed())
			Expect(server.InstallIFixes(wlpPath, ifixes, executor, bard.NewLogger(io.Discard))).To(Succeed())

			execution := executor.Calls[0].Arguments[0].(effect.Execution)
			Expect(execution.Command).To(Equal("java"))
			Expect(execution.Args).To(Equal([]string{"-jar", ifixes[0], "--installLocation", wlpPath}))

			execution = executor.Calls[1].Arguments[0].(effect.Execution)
			Expect(execution.Command).To(Equal("java"))
			Expect(execution.Args).To(Equal([]string{"-jar", ifixes[1], "--installLocation", wlpPath}))
		})
	})

	when("installing features", func() {
		it("works", func() {
			executor := &mocks.Executor{}
			executor.On("Execute", mock.Anything).Return(nil)
			features := []string{"foo", "bar", "baz"}
			Expect(server.InstallFeatures(wlpPath, features, executor, bard.NewLogger(io.Discard)))
			for i, call := range executor.Calls {
				execution := call.Arguments[0].(effect.Execution)
				Expect(execution.Command).To(Equal(filepath.Join(wlpPath, "bin", "featureUtility")))
				Expect(execution.Args).To(Equal([]string{"installFeature", features[i], "--acceptLicense"}))
			}
		})
	})
}
