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

func testApp(t *testing.T, when spec.G, it spec.S) {
	var (
		Expect   = NewWithT(t).Expect
		testPath string
	)

	it.Before(func() {
		var err error
		testPath, err = os.MkdirTemp("", "app-utils")
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

	when("checking a path for compiled artifacts", func() {
		it("finds a war", func() {
			Expect(os.WriteFile(filepath.Join(testPath, "app.war"), []byte{}, 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(testPath, "server.xml"), []byte{}, 0644)).To(Succeed())
			appList, err := util.GetApps(testPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(appList).To(Equal([]string{filepath.Join(testPath, "app.war")}))
		})

		it("finds an ear", func() {
			Expect(os.WriteFile(filepath.Join(testPath, "app.ear"), []byte{}, 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(testPath, "server.xml"), []byte{}, 0644)).To(Succeed())
			appList, err := util.GetApps(testPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(appList).To(Equal([]string{filepath.Join(testPath, "app.ear")}))
		})

		it("returns the empty list if path looks like an expanded EAR", func() {
			Expect(os.Mkdir(filepath.Join(testPath, "META-INF"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(testPath, "META-INF", "application.xml"), []byte{}, 0644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(testPath, "app.war"), []byte{}, 0644)).To(Succeed())
			appList, err := util.GetApps(testPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(appList).To(BeEmpty())
		})
	})
}
