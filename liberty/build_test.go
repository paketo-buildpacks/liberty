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

package liberty_test

import (
	"bytes"
	"github.com/paketo-buildpacks/libpak"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/liberty/liberty"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/sclevine/spec"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect
		ctx    libcnb.BuildContext
	)

	it.Before(func() {
		var err error

		ctx.Application.Path, err = ioutil.TempDir("", "build-application")
		Expect(err).NotTo(HaveOccurred())

		ctx.Buildpack.Metadata = map[string]interface{}{
			"configurations": []map[string]interface{}{
				{"name": "BP_LIBERTY_VERSION", "default": "21.0.11", "build": true},
				{"name": "BP_LIBERTY_PROFILE", "default": "full", "build": true},
				{"name": "BP_LIBERTY_INSTALL_TYPE", "default": "ol", "build": true},
				{"name": "BP_LIBERTY_SERVER_NAME", "default": "", "build": true},
			},
			"dependencies": []map[string]interface{}{
				{"id": "open-liberty-runtime-full", "version": "21.0.11"},
				{"id": "open-liberty-runtime-microProfile4", "version": "21.0.10"},
			},
		}

		ctx.Plan.Entries = []libcnb.BuildpackPlanEntry{{Name: "liberty", Metadata: map[string]interface{}{
			"server-name": "defaultServer",
		}}}

		ctx.Layers.Path, err = ioutil.TempDir("", "build-layers")
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
	})

	it("picks the latest full profile when no arguments are set", func() {
		Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "WEB-INF"), 0755)).To(Succeed())

		result, err := liberty.Build{Logger: bard.NewLogger(io.Discard)}.Build(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Layers).To(HaveLen(3))
		Expect(result.Layers[0].Name()).To(Equal("helper"))
		Expect(result.Layers[1].Name()).To(Equal("open-liberty-runtime-full"))
		Expect(result.Layers[2].Name()).To(Equal("base"))
	})
	
	context("requested app server is not liberty", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_JAVA_APP_SERVER", "notliberty")).To(Succeed())
		})
		
		it.After(func() {
			Expect(os.Unsetenv("BP_JAVA_APP_SERVER")).To(Succeed())
		})
		
		it("should not run if liberty is not the requested java app server", func() {
			ctx.Plan.Entries = []libcnb.BuildpackPlanEntry{{Name: "test"}}

			buf := &bytes.Buffer{}
			result, err := liberty.Build{Logger: bard.NewLogger(buf)}.Build(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers).To(HaveLen(0))
			Expect(result.Unmet).To(ContainElement(libcnb.UnmetPlanEntry{Name: "test"}))
		})
	})
	
	context("missing required info", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_DEBUG", "true")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_DEBUG")).To(Succeed())
		})

		context("Main-Class in MANIFEST.MF", func() {
			it.Before(func() {
				Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "META-INF"), 0755)).To(Succeed())
				Expect(ioutil.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "MANIFEST.MF"), []byte(`Main-Class: org.DoStuff`), 0644)).To(Succeed())
			})

			it("doesn't run", func() {
				buf := &bytes.Buffer{}
				result, err := liberty.Build{Logger: bard.NewLogger(buf)}.Build(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(result.Layers).To(HaveLen(0))
				Expect(result.Unmet).To(ContainElement(libcnb.UnmetPlanEntry{Name: "liberty"}))

				Expect(buf.String()).To(ContainSubstring("`Main-Class` found in `META-INF/MANIFEST.MF`, skipping build\n"))
			})
		})

		context("missing WEB-INF and application.xml", func() {
			it("doesn't run", func() {
				buf := &bytes.Buffer{}
				result, err := liberty.Build{Logger: bard.NewLogger(buf)}.Build(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(result.Layers).To(HaveLen(0))
				Expect(result.Unmet).To(ContainElement(libcnb.UnmetPlanEntry{Name: "liberty"}))

				Expect(buf.String()).To(ContainSubstring("No `WEB-INF/` or `META-INF/application.xml` found, skipping build\n"))
			})
		})
	})

	context("user env config set", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_LIBERTY_VERSION", "21.0.10")).To(Succeed())
			Expect(os.Setenv("BP_LIBERTY_PROFILE", "microProfile4")).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "META-INF"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "application.xml"), []byte{}, 0644)).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_LIBERTY_VERSION")).To(Succeed())
			Expect(os.Unsetenv("BP_LIBERTY_PROFILE")).To(Succeed())
		})

		it("honors user set configuration values", func() {
			result, err := liberty.Build{Logger: bard.NewLogger(io.Discard)}.Build(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers).To(HaveLen(3))
			Expect(result.Layers[0].Name()).To(Equal("helper"))
			Expect(result.Layers[1].Name()).To(Equal("open-liberty-runtime-microProfile4"))
			Expect(result.Layers[2].Name()).To(Equal("base"))
		})
	})

	context("$BP_LIBERTY_EXT_CONF_URI", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_LIBERTY_EXT_CONF_SHA256", "test-sha256")).To(Succeed())
			Expect(os.Setenv("BP_LIBERTY_EXT_CONF_URI", "test-uri")).To(Succeed())
			Expect(os.Setenv("BP_LIBERTY_EXT_CONF_VERSION", "test-version")).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_LIBERTY_EXT_CONF_SHA256")).To(Succeed())
			Expect(os.Unsetenv("BP_LIBERTY_EXT_CONF_URI")).To(Succeed())
			Expect(os.Unsetenv("BP_LIBERTY_EXT_CONF_VERSION")).To(Succeed())
		})

		it("contributes external configuration when $BP_LIBERTY_EXT_CONF_URI is set", func() {
			Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "WEB-INF"), 0755)).To(Succeed())

			ctx.Buildpack.Metadata = map[string]interface{}{
				"configurations": []map[string]interface{}{
					{"name": "BP_LIBERTY_VERSION", "default": "21.0.11", "build": true},
					{"name": "BP_LIBERTY_PROFILE", "default": "full", "build": true},
					{"name": "BP_LIBERTY_INSTALL_TYPE", "default": "ol", "build": true},
					{"name": "BP_LIBERTY_SERVER_NAME", "default": "", "build": true},
				},
				"dependencies": []map[string]interface{}{
					{
						"cpes":    "cpe:2.3:a:ibm:liberty:21.0.0.11:*:*:*:*:*:*:*:*",
						"id":      "open-liberty-runtime-full",
						"name":    "Open Liberty (All Features)",
						"purl":    "pkg:generic/ibm-open-libery-runtime-full@21.0.0.11?arch=amd64",
						"stacks":  []interface{}{"test-stack-id"},
						"version": "21.0.11",
					},
				},
			}
			ctx.StackID = "test-stack-id"

			result, err := liberty.Build{}.Build(ctx)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers).To(HaveLen(3))
			Expect(result.Layers[0].Name()).To(Equal("helper"))
			Expect(result.Layers[1].Name()).To(Equal("open-liberty-runtime-full"))
			Expect(result.Layers[2].Name()).To(Equal("base"))
			Expect(result.Layers[2].(liberty.Base).ExternalConfigurationDependency).To(Equal(&libpak.BuildpackDependency{
				ID:      "open-liberty-external-configuration",
				Name:    "Open Liberty External Configuration",
				Version: "test-version",
				URI:     "test-uri",
				SHA256:  "test-sha256",
				Stacks:  []string{ctx.StackID},
			}))
		})
	})

	context("when building a packaged server", func() {
		it.Before(func() {
			usrPath := filepath.Join(ctx.Application.Path, "usr")
			Expect(os.MkdirAll(filepath.Join(usrPath, "servers", "defaultServer", "apps", "test.war"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(usrPath, "servers", "defaultServer", "server.xml"), []byte("<server/>"), 0644)).To(Succeed())
			ctx.Plan.Entries = []libcnb.BuildpackPlanEntry{{Name: "liberty", Metadata: map[string]interface{}{
				"server-name":              "defaultServer",
				"packaged-server-usr-path": usrPath,
			}}}
			Expect(os.Setenv("BP_DEBUG", "true"))
		})

		it.After(func() {
			Expect(os.RemoveAll(filepath.Join(ctx.Application.Path, "usr"))).To(Succeed())
		})

		it("should discover the app", func() {
			result, err := liberty.Build{Logger: bard.NewLogger(io.Discard)}.Build(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Layers).To(HaveLen(3))
			Expect(result.Layers[0].Name()).To(Equal("helper"))
			Expect(result.Layers[1].Name()).To(Equal("open-liberty-runtime-full"))
			Expect(result.Layers[2].Name()).To(Equal("base"))
			Expect(result.Unmet).To(HaveLen(0))
		})

		it("should not run if no apps are installed", func() {
			Expect(os.RemoveAll(filepath.Join(ctx.Application.Path, "usr", "servers", "defaultServer", "apps", "test.war"))).To(Succeed())
			result, err := liberty.Build{Logger: bard.NewLogger(io.Discard)}.Build(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Unmet).To(HaveLen(1))
			Expect(result.Unmet).To(ContainElement(libcnb.UnmetPlanEntry{Name: "liberty"}))
		})
	})
}
