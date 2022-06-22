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
	"fmt"
	"github.com/paketo-buildpacks/liberty/internal/util"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/liberty/liberty"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBase(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		ctx libcnb.BuildContext
	)

	it.Before(func() {
		var err error
		ctx.Layers.Path, err = ioutil.TempDir("", "base-layers")
		Expect(err).NotTo(HaveOccurred())
		ctx.Layers.Path, err = filepath.EvalSymlinks(ctx.Layers.Path)
		Expect(err).ToNot(HaveOccurred())

		ctx.Application.Path, err = ioutil.TempDir("", "workspace")
		Expect(err).ToNot(HaveOccurred())
		ctx.Application.Path, err = filepath.EvalSymlinks(ctx.Application.Path)
		Expect(err).ToNot(HaveOccurred())

		ctx.Buildpack.Path, err = ioutil.TempDir("", "base-buildpack")
		Expect(err).ToNot(HaveOccurred())
		ctx.Buildpack.Path, err = filepath.EvalSymlinks(ctx.Buildpack.Path)
		Expect(err).ToNot(HaveOccurred())

		ctx.Buildpack.Metadata = map[string]interface{}{
			"configurations": []map[string]interface{}{
				{"name": "BP_LIBERTY_SERVER_NAME", "default": "defaultServer"},
			},
		}
		srcTemplateDir := filepath.Join(ctx.Buildpack.Path, "templates")
		Expect(os.Mkdir(srcTemplateDir, 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(srcTemplateDir, "app.tmpl"), []byte{}, 0644)).To(Succeed())
		template := `<?xml version="1.0" encoding="UTF-8"?>
<server>
  <featureManager>
  {{ range $val := . }}
    <feature>{{ $val }}</feature>
  {{ end }}
  </featureManager>
</server>`
		Expect(os.WriteFile(filepath.Join(srcTemplateDir, "server.tmpl"), []byte(template), 0644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(srcTemplateDir, "expose-default-endpoint.xml"), []byte{}, 0644)).To(Succeed())
	})

	it.After(func() {
		Expect(os.RemoveAll(ctx.Layers.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Buildpack.Path)).To(Succeed())
		Expect(os.RemoveAll(ctx.Application.Path)).To(Succeed())
	})

	it("contributes the config templates and app", func() {
		// Create app
		Expect(os.Mkdir(filepath.Join(ctx.Application.Path, "WEB-INF"), 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "WEB-INF", "web.xml"), []byte{}, 0644))

		dc := libpak.DependencyCache{CachePath: "testdata"}
		base := liberty.NewBase(ctx.Application.Path, ctx.Buildpack.Path, "defaultServer", "kernel", nil, nil, libpak.ConfigurationResolver{}, dc, nil)
		base.Logger = bard.NewLogger(os.Stdout)
		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = base.Contribute(layer)
		Expect(err).ToNot(HaveOccurred())

		Expect(layer.Launch).To(BeTrue())

		// Check for templates
		Expect(filepath.Join(layer.Path, "templates")).To(BeADirectory())
		Expect(filepath.Join(layer.Path, "templates", "app.tmpl")).To(BeARegularFile())
		Expect(filepath.Join(layer.Path, "templates", "expose-default-endpoint.xml")).To(BeARegularFile())
		Expect(filepath.Join(layer.Path, "templates", "server.tmpl")).To(BeARegularFile())

		// Check app
		appPath, err := filepath.EvalSymlinks(filepath.Join(filepath.Join(layer.Path, "wlp", "usr", "servers", "defaultServer", "apps", "app")))
		Expect(err).ToNot(HaveOccurred())
		Expect(appPath).To(Equal(ctx.Application.Path))
		Expect(filepath.Join(appPath, "WEB-INF", "web.xml")).To(BeARegularFile())

		// Evaluate WLP_USR_DIR symlink to handle macOS temporary path
		wlpUserDir := layer.LaunchEnvironment["WLP_USER_DIR.default"]
		wlpUserDir, err = filepath.EvalSymlinks(wlpUserDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(wlpUserDir).To(Equal(filepath.Join(layer.Path, "wlp", "usr")))
	})

	it("contributes a default server.xml", func() {
		dc := libpak.DependencyCache{CachePath: "testdata"}
		base := liberty.NewBase(ctx.Application.Path, ctx.Buildpack.Path, "defaultServer", "kernel", nil, nil, libpak.ConfigurationResolver{}, dc, nil)
		base.Logger = bard.NewLogger(os.Stdout)
		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = base.Contribute(layer)
		Expect(err).ToNot(HaveOccurred())

		xmlFile, err := os.Open(filepath.Join(layer.Path, "wlp", "usr", "servers", "defaultServer", "server.xml"))
		Expect(err).ToNot(HaveOccurred())
		defer xmlFile.Close()

		bytes, err := ioutil.ReadAll(xmlFile)
		Expect(err).ToNot(HaveOccurred())
		fmt.Println(string(bytes))
		Expect(string(bytes)).To(Equal(`<?xml version="1.0" encoding="UTF-8"?>
<server>
  <featureManager>
  
    <feature>jsp-2.3</feature>
  
  </featureManager>
</server>`))
	})

	it("contributes features to server.xml", func() {
		dc := libpak.DependencyCache{CachePath: "testdata"}
		base := liberty.NewBase(ctx.Application.Path, ctx.Buildpack.Path, "defaultServer", "kernel", []string{"jaxrs-2.1", "cdi-2.0"}, nil, libpak.ConfigurationResolver{}, dc, nil)
		base.Logger = bard.NewLogger(os.Stdout)
		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = base.Contribute(layer)
		Expect(err).ToNot(HaveOccurred())

		xmlFile, err := os.Open(filepath.Join(layer.Path, "wlp", "usr", "servers", "defaultServer", "server.xml"))
		Expect(err).ToNot(HaveOccurred())
		defer xmlFile.Close()

		bytes, err := ioutil.ReadAll(xmlFile)
		Expect(err).ToNot(HaveOccurred())
		fmt.Println(string(bytes))
		Expect(string(bytes)).To(Equal(`<?xml version="1.0" encoding="UTF-8"?>
<server>
  <featureManager>
  
    <feature>jaxrs-2.1</feature>
  
    <feature>cdi-2.0</feature>
  
  </featureManager>
</server>`))
	})

	it("contributes server.xml from external configuration", func() {
		externalConfigurationDep := libpak.BuildpackDependency{
			ID:     "open-liberty-external-configuration",
			URI:    "https://localhost/stub-external-configuration-with-directory.tar.gz",
			SHA256: "6be374a1c3eeda98effb66fe7f9feb48ffcf325492237bc717c77794566f8ce0",
			PURL:   "pkg:generic/ibm-open-libery-runtime-full@21.0.0.11?arch=amd64",
			CPEs:   []string{"cpe:2.3:a:ibm:liberty:21.0.0.11:*:*:*:*:*:*:*:*"},
		}
		dc := libpak.DependencyCache{CachePath: "testdata"}
		base := liberty.NewBase(ctx.Application.Path, ctx.Buildpack.Path, "defaultServer", "kernel", nil, &externalConfigurationDep, libpak.ConfigurationResolver{}, dc, nil)
		base.Logger = bard.NewLogger(os.Stdout)
		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = base.Contribute(layer)
		Expect(err).ToNot(HaveOccurred())

		xmlFile, err := os.Open(filepath.Join(layer.Path, "conf", "external-configuration", "server.xml"))
		Expect(err).ToNot(HaveOccurred())
		defer xmlFile.Close()

		bytes, err := ioutil.ReadAll(xmlFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(bytes)).To(Equal("<server description=\"stub server\"/>\n"))
	})

	it("contributes server.xml and compiled artifact", func() {
		// Set up app and server config
		Expect(util.CopyFile(filepath.Join("testdata", "test.war"), filepath.Join(ctx.Application.Path, "test.war"))).To(Succeed())
		Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "server.xml"), []byte("<server/>"), 0644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "server.env"), []byte("TEST_ENV=foo"), 0644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(ctx.Application.Path, "bootstrap.properties"), []byte("test.property=foo"), 0644)).To(Succeed())

		dc := libpak.DependencyCache{CachePath: "testdata"}
		base := liberty.NewBase(ctx.Application.Path, ctx.Buildpack.Path, "defaultServer", "kernel", nil, nil, libpak.ConfigurationResolver{}, dc, nil)
		base.Logger = bard.NewLogger(os.Stdout)
		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = base.Contribute(layer)
		Expect(err).ToNot(HaveOccurred())

		Expect(filepath.Join(layer.Path, "wlp", "usr", "servers", "defaultServer", "apps", "app")).To(BeADirectory())
		Expect(filepath.Join(layer.Path, "wlp", "usr", "servers", "defaultServer", "apps", "app", "index.html")).To(BeAnExistingFile())

		serverDir := filepath.Join(layer.Path, "wlp", "usr", "servers", "defaultServer")
		for _, file := range []string{
			"server.xml",
			"server.env",
			"bootstrap.properties",
		} {
			resolved, err := filepath.EvalSymlinks(filepath.Join(serverDir, file))
			Expect(err).ToNot(HaveOccurred())
			Expect(resolved).To(Equal(filepath.Join(ctx.Application.Path, file)))
		}
	})

	it("sets the WLP_USER_DIR to packaged server's wlp directory", func() {
		// Create packaged server
		serverSourcePath := filepath.Join(ctx.Application.Path, "wlp", "usr", "servers", "defaultServer")
		Expect(os.MkdirAll(serverSourcePath, 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(serverSourcePath, "server.xml"), []byte("<server />"), 0644))

		dc := libpak.DependencyCache{CachePath: "testdata"}
		base := liberty.NewBase(ctx.Application.Path, ctx.Buildpack.Path, "defaultServer", "kernel", nil, nil, libpak.ConfigurationResolver{}, dc, nil)
		base.Logger = bard.NewLogger(os.Stdout)
		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = base.Contribute(layer)
		Expect(err).ToNot(HaveOccurred())

		// Evaluate WLP_USR_DIR symlink to handle macOS temporary path
		wlpUserDir := layer.LaunchEnvironment["WLP_USER_DIR.default"]
		wlpUserDir, err = filepath.EvalSymlinks(wlpUserDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(wlpUserDir).To(Equal(filepath.Join(ctx.Application.Path, "wlp", "usr")))
	})

	it("sets the WLP_USER_DIR to packaged server's usr directory", func() {
		// Create packaged server
		serverSourcePath := filepath.Join(ctx.Application.Path, "usr", "servers", "defaultServer")
		Expect(os.MkdirAll(serverSourcePath, 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(serverSourcePath, "server.xml"), []byte("<server />"), 0644))

		dc := libpak.DependencyCache{CachePath: "testdata"}
		base := liberty.NewBase(ctx.Application.Path, ctx.Buildpack.Path, "defaultServer", "kernel", nil, nil, libpak.ConfigurationResolver{}, dc, nil)
		base.Logger = bard.NewLogger(os.Stdout)
		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = base.Contribute(layer)
		Expect(err).ToNot(HaveOccurred())

		// Evaluate WLP_USR_DIR symlink to handle macOS temporary path
		wlpUserDir := layer.LaunchEnvironment["WLP_USER_DIR.default"]
		wlpUserDir, err = filepath.EvalSymlinks(wlpUserDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(wlpUserDir).To(Equal(filepath.Join(ctx.Application.Path, "usr")))
	})

	it("automatically finds server name", func() {
		// Create packaged server
		serverSourcePath := filepath.Join(ctx.Application.Path, "usr", "servers", "testServer")
		Expect(os.MkdirAll(serverSourcePath, 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(serverSourcePath, "server.xml"), []byte("<server />"), 0644))

		dc := libpak.DependencyCache{CachePath: "testdata"}
		base := liberty.NewBase(ctx.Application.Path, ctx.Buildpack.Path, "testServer", "kernel", nil, nil, libpak.ConfigurationResolver{}, dc, nil)
		base.Logger = bard.NewLogger(os.Stdout)
		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		layer, err = base.Contribute(layer)
		Expect(err).ToNot(HaveOccurred())

		// Evaluate WLP_USR_DIR symlink to
		wlpUserDir := layer.LaunchEnvironment["WLP_USER_DIR.default"]
		wlpUserDir, err = filepath.EvalSymlinks(wlpUserDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(wlpUserDir).To(Equal(filepath.Join(ctx.Application.Path, "usr")))
	})

	it("installs user features and creates the features config", func() {
		template := `<?xml version="1.0" encoding="UTF-8"?>
                         <server>
                           <!-- Enable user features -->
                           <featureManager>
                             {{ range $val := . }}
                                 <feature>{{ $val }}</feature>
                             {{ end }}
                           </featureManager>
                         </server>`
		Expect(os.WriteFile(filepath.Join(ctx.Buildpack.Path, "templates", "features.tmpl"), []byte(template), 0644)).To(Succeed())

		layer, err := ctx.Layers.Layer("test-layer")
		Expect(err).NotTo(HaveOccurred())

		externalConfigurationDep := libpak.BuildpackDependency{
			ID:     "open-liberty-external-configuration",
			URI:    "https://localhost/stub-user-feature-configuration.tar.gz",
			SHA256: "ea4dfc314869b9b6969b5e0122c038d2c0ec87bd8c3f0b3ff39ad27e73fee336",
			PURL:   "pkg:generic/ibm-open-libery-runtime-full@21.0.0.11?arch=amd64",
			CPEs:   []string{"cpe:2.3:a:ibm:liberty:21.0.0.11:*:*:*:*:*:*:*:*"},
		}

		// Contribute the layer
		dc := libpak.DependencyCache{CachePath: "testdata"}
		base := liberty.NewBase(ctx.Application.Path, ctx.Buildpack.Path, "defaultServer", "kernel", nil, &externalConfigurationDep, libpak.ConfigurationResolver{}, dc, nil)
		base.Logger = bard.NewLogger(os.Stdout)

		layer, err = base.Contribute(layer)
		Expect(err).ToNot(HaveOccurred())

		// Check that the feature was installed
		usrPath := filepath.Join(layer.Path, "wlp", "usr")
		Expect(filepath.Join(usrPath, "extension", "lib", "test.feature_1.0.0.jar")).To(BeARegularFile())
		Expect(filepath.Join(usrPath, "extension", "lib", "features", "test.feature_1.0.0.mf")).To(BeARegularFile())
		Expect(filepath.Join(usrPath, "servers", "defaultServer", "configDropins", "defaults", "features.xml")).To(BeARegularFile())
	})
}
