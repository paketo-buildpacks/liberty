/*
 * Copyright 2018-2020 the original author or authors.
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

package openliberty_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/open-liberty/openliberty"
	"github.com/sclevine/spec"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx     libcnb.BuildContext
		builder openliberty.Build
	)

	it.Before(func() {
		var err error

		ctx.Application.Path, err = ioutil.TempDir("", "build-application")
		Expect(err).NotTo(HaveOccurred())

		ctx.Buildpack.Metadata = map[string]interface{}{
			"configurations": []map[string]interface{}{
				{"name": "BP_OPENLIBERTY_VERSION", "default": "21.0.11"},
				{"name": "BP_OPENLIBERTY_PROFILE", "default": "full"},
			},
			"dependencies": []map[string]interface{}{
				{"id": "open-liberty-runtime-full", "version": "21.0.11"},
				{"id": "open-liberty-runtime-microProfile4", "version": "21.0.10"},
			},
		}

		ctx.Layers.Path, err = ioutil.TempDir("", "build-layers")
		Expect(err).NotTo(HaveOccurred())
		builder = openliberty.Build{}
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
	})

	it("picks the latest full profile when no arguments are set", func() {
		Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "WEB-INF"), 0755)).To(Succeed())

		buf := &bytes.Buffer{}
		builder.Logger = bard.NewLogger(buf)

		_, err := builder.Build(ctx)
		Expect(err).NotTo(HaveOccurred())

		output := buf.String()
		Expect(output).To(ContainSubstring("Choosing default version 21.0.11 for Open Liberty runtime"))
		Expect(output).To(ContainSubstring("Choosing default profile full for Open Liberty runtime"))
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
				ctx.Plan.Entries = []libcnb.BuildpackPlanEntry{{Name: "test"}}

				buf := &bytes.Buffer{}
				builder.Logger = bard.NewLogger(buf)

				result, err := builder.Build(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(buf.String()).To(Equal("`Main-Class` found in `META-INF/MANIFEST.MF`, skipping build\n"))
				Expect(result.Unmet).To(ContainElement(libcnb.UnmetPlanEntry{Name: "test"}))
			})
		})

		context("missing WEB-INF and application.xml", func() {
			it("doesn't run", func() {
				ctx.Plan.Entries = []libcnb.BuildpackPlanEntry{{Name: "test"}}

				buf := &bytes.Buffer{}
				builder.Logger = bard.NewLogger(buf)

				result, err := builder.Build(ctx)
				Expect(err).NotTo(HaveOccurred())

				Expect(buf.String()).To(Equal("No `WEB-INF/` or `META-INF/application.xml` found, skipping build\n"))
				Expect(result.Unmet).To(ContainElement(libcnb.UnmetPlanEntry{Name: "test"}))
			})
		})
	})

	context("user env config set", func() {
		it.Before(func() {
			Expect(os.Setenv("BP_OPENLIBERTY_VERSION", "21.0.10")).To(Succeed())
			Expect(os.Setenv("BP_OPENLIBERTY_PROFILE", "microProfile4")).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "META-INF"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(ctx.Application.Path, "META-INF", "application.xml"), []byte{}, 0644)).To(Succeed())
		})

		it.After(func() {
			Expect(os.Unsetenv("BP_OPENLIBERTY_VERSION")).To(Succeed())
			Expect(os.Unsetenv("BP_OPENLIBERTY_PROFILE")).To(Succeed())
		})

		it("honors user set configuration values", func() {
			buf := &bytes.Buffer{}
			builder.Logger = bard.NewLogger(buf)

			_, err := builder.Build(ctx)
			Expect(err).NotTo(HaveOccurred())

			output := buf.String()
			Expect(output).To(ContainSubstring("Choosing user-defined version 21.0.10 for Open Liberty runtime"))
			Expect(output).To(ContainSubstring("Choosing user-defined profile microProfile4 for Open Liberty runtime"))
		})
	})

}
