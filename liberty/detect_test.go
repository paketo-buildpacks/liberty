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

package liberty_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/liberty/liberty"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testDetect(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx    libcnb.DetectContext
		detect liberty.Detect
	)

	it.Before(func() {
		var err error

		ctx.Application.Path, err = ioutil.TempDir("", "open-liberty-app")
		Expect(err).NotTo(HaveOccurred())

		ctx.Platform.Path, err = ioutil.TempDir("", "open-liberty-test-platform")
		Expect(err).NotTo(HaveOccurred())

		ctx.Buildpack.Metadata = map[string]interface{}{
			"configurations": []map[string]interface{}{
				{"name": "BP_LIBERTY_SERVER_NAME", "default": "defaultServer"},
			},
		}

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
						{Name: liberty.PlanEntryLiberty},
					},

					Requires: []libcnb.BuildPlanRequire{
						{Name: liberty.PlanEntryJRE, Metadata: map[string]interface{}{
							"launch": true,
							"build":  true,
							"cache":  true,
						}},
						{Name: liberty.PlanEntryJVMApplicationPackage},
						{Name: liberty.PlanEntryLiberty},
						{Name: liberty.PlanEntryJavaAppServer},
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
						{Name: liberty.PlanEntryLiberty},
						{Name: liberty.PlanEntryJVMApplicationPackage},
					},

					Requires: []libcnb.BuildPlanRequire{
						{Name: liberty.PlanEntryJRE, Metadata: map[string]interface{}{
							"launch": true,
							"build":  true,
							"cache":  true,
						}},
						{Name: liberty.PlanEntryJVMApplicationPackage},
						{Name: liberty.PlanEntryLiberty},
						{Name: liberty.PlanEntryJavaAppServer},
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
						{Name: liberty.PlanEntryLiberty},
						{Name: liberty.PlanEntryJVMApplicationPackage},
					},

					Requires: []libcnb.BuildPlanRequire{
						{Name: liberty.PlanEntryJRE, Metadata: map[string]interface{}{
							"launch": true,
							"build":  true,
							"cache":  true,
						}},
						{Name: liberty.PlanEntryJVMApplicationPackage},
						{Name: liberty.PlanEntryLiberty},
						{Name: liberty.PlanEntryJavaAppServer},
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
						{Name: liberty.PlanEntryLiberty},
						{Name: liberty.PlanEntryJVMApplicationPackage},
					},

					Requires: []libcnb.BuildPlanRequire{
						{Name: liberty.PlanEntryJRE, Metadata: map[string]interface{}{
							"launch": true,
							"build":  true,
							"cache":  true,
						}},
						{Name: liberty.PlanEntryJVMApplicationPackage},
						{Name: liberty.PlanEntryLiberty},
						{Name: liberty.PlanEntryJavaAppServer},
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

	context("when building a packaged server", func() {
		it.Before(func() {
			Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "wlp", "usr", "servers", "defaultServer", "apps", "test.war"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "wlp", "usr", "servers", "defaultServer", "server.xml"), []byte("<server/>"), 0644)).To(Succeed())
		})

		it.After(func() {
			Expect(os.RemoveAll(filepath.Join(ctx.Application.Path, "wlp"))).To(Succeed())
		})

		it("works", func() {
			result, err := detect.Detect(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(libcnb.DetectResult{
				Pass: true,
				Plans: []libcnb.BuildPlan{
					{
						Provides: []libcnb.BuildPlanProvide{
							{Name: liberty.PlanEntryLiberty},
							{Name: liberty.PlanEntryJVMApplicationPackage},
						},

						Requires: []libcnb.BuildPlanRequire{
							{Name: liberty.PlanEntryJRE, Metadata: map[string]interface{}{
								"launch": true,
								"build":  true,
								"cache":  true,
							}},
							{Name: liberty.PlanEntryJavaAppServer},
							{Name: liberty.PlanEntryJVMApplicationPackage},
							{Name: liberty.PlanEntryLiberty, Metadata: map[string]interface{}{
								"packaged-server":          true,
								"packaged-server-usr-path": filepath.Join(ctx.Application.Path, "wlp", "usr"),
							}},
						},
					},
				},
			}))
		})

		it("does not provide jvm-application-package if packaged server has no apps", func() {
			Expect(os.RemoveAll(filepath.Join(ctx.Application.Path, "wlp", "usr", "servers", "defaultServer", "apps", "test.war"))).To(Succeed())
			result, err := detect.Detect(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(libcnb.DetectResult{
				Pass: true,
				Plans: []libcnb.BuildPlan{
					{
						Provides: []libcnb.BuildPlanProvide{
							{Name: liberty.PlanEntryLiberty},
						},

						Requires: []libcnb.BuildPlanRequire{
							{Name: liberty.PlanEntryJRE, Metadata: map[string]interface{}{
								"launch": true,
								"build":  true,
								"cache":  true,
							}},
							{Name: liberty.PlanEntryJavaAppServer},
							{Name: liberty.PlanEntryJVMApplicationPackage},
							{Name: liberty.PlanEntryLiberty, Metadata: map[string]interface{}{
								"packaged-server":          true,
								"packaged-server-usr-path": filepath.Join(ctx.Application.Path, "wlp", "usr"),
							}},
						},
					},
				},
			}))
		})
	})

	context("when building a server directory", func() {
		it.Before(func() {
			Expect(os.MkdirAll(filepath.Join(ctx.Application.Path, "usr", "servers", "defaultServer", "apps", "test.war"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "usr", "servers", "defaultServer", "server.xml"), []byte("<server/>"), 0644)).To(Succeed())
		})

		it.After(func() {
			Expect(os.RemoveAll(filepath.Join(ctx.Application.Path, "usr"))).To(Succeed())
		})

		it("works", func() {
			result, err := detect.Detect(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(libcnb.DetectResult{
				Pass: true,
				Plans: []libcnb.BuildPlan{
					{
						Provides: []libcnb.BuildPlanProvide{
							{Name: liberty.PlanEntryLiberty},
							{Name: liberty.PlanEntryJVMApplicationPackage},
						},

						Requires: []libcnb.BuildPlanRequire{
							{Name: liberty.PlanEntryJRE, Metadata: map[string]interface{}{
								"launch": true,
								"build":  true,
								"cache":  true,
							}},
							{Name: liberty.PlanEntryJavaAppServer},
							{Name: liberty.PlanEntryJVMApplicationPackage},
							{Name: liberty.PlanEntryLiberty, Metadata: map[string]interface{}{
								"packaged-server":          true,
								"packaged-server-usr-path": filepath.Join(ctx.Application.Path, "usr"),
							}},
						},
					},
				},
			}))
		})

		it("does not provide jvm-application-package if packaged server has no apps", func() {
			Expect(os.RemoveAll(filepath.Join(ctx.Application.Path, "usr", "servers", "defaultServer", "apps", "test.war"))).To(Succeed())
			result, err := detect.Detect(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(libcnb.DetectResult{
				Pass: true,
				Plans: []libcnb.BuildPlan{
					{
						Provides: []libcnb.BuildPlanProvide{
							{Name: liberty.PlanEntryLiberty},
						},

						Requires: []libcnb.BuildPlanRequire{
							{Name: liberty.PlanEntryJRE, Metadata: map[string]interface{}{
								"launch": true,
								"build":  true,
								"cache":  true,
							}},
							{Name: liberty.PlanEntryJavaAppServer},
							{Name: liberty.PlanEntryJVMApplicationPackage},
							{Name: liberty.PlanEntryLiberty, Metadata: map[string]interface{}{
								"packaged-server":          true,
								"packaged-server-usr-path": filepath.Join(ctx.Application.Path, "usr"),
							}},
						},
					},
				},
			}))
		})
	})
}
