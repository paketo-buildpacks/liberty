package core_test

import (
	"github.com/paketo-buildpacks/liberty/internal/core"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/sclevine/spec"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func testBuildSource(t *testing.T, when spec.G, it spec.S) {
	var (
		Expect   = NewWithT(t).Expect
		testPath string
	)

	it.Before(func() {
		var err error
		testPath, err = ioutil.TempDir("", "core")
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(testPath)).To(Succeed())
	})

	it("TEst", func() {
		Expect(testPath).ToNot(BeEmpty())
	})

	when("building an app source", func() {
		it("detects when Main-Class set and skips the build", func() {
			Expect(os.Mkdir(filepath.Join(testPath, "META-INF"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(testPath, "META-INF", "MANIFEST.MF"),
				[]byte("Main-Class: com.java.HelloWorld"),
				0644)).To(Succeed())
			appBuildSource := core.NewAppBuildSource(testPath, bard.NewLogger(ioutil.Discard))
			ok, err := appBuildSource.Detect()
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		it("detects successfully when Main-Class is not set", func() {
			appBuildSrc := core.NewAppBuildSource(testPath, bard.NewLogger(ioutil.Discard))
			ok, err := appBuildSrc.Detect()
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		it("validates successfully when a compiled web archive is supplied", func() {
			Expect(os.Mkdir(filepath.Join(testPath, "WEB-INF"), 0755)).To(Succeed())
			appBuildSrc := core.NewAppBuildSource(testPath, bard.NewLogger(ioutil.Discard))
			ok, err := appBuildSrc.ValidateApp()
			Expect(err).To(Succeed())
			Expect(ok).To(BeTrue())
		})

		it("validates successfully when a compiled enterprise archive is supplied", func() {
			Expect(os.Mkdir(filepath.Join(testPath, "META-INF"), 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(testPath, "META-INF", "application.xml"), []byte{}, 0644)).To(Succeed())
			appBuildSrc := core.NewAppBuildSource(testPath, bard.NewLogger(ioutil.Discard))
			ok, err := appBuildSrc.ValidateApp()
			Expect(err).To(Succeed())
			Expect(ok).To(BeTrue())
		})

		it("fails app validation when META-INF or application.xml not found", func() {
			appBuildSrc := core.NewAppBuildSource(testPath, bard.NewLogger(ioutil.Discard))
			ok, err := appBuildSrc.ValidateApp()
			Expect(err).To(Succeed())
			Expect(ok).To(BeFalse())
		})
	})

	when("building a server source", func() {
		when("wlp is provided", func() {
			var serversPath string

			it.Before(func() {
				serversPath = filepath.Join(testPath, "wlp", "usr", "servers")
				Expect(os.MkdirAll(serversPath, 0755)).To(Succeed())
			})

			it.After(func() {
				Expect(os.RemoveAll(serversPath)).To(Succeed())
			})

			it("detects successfully if there is a server named defaultServer", func() {
				defaultServerPath := filepath.Join(serversPath, "defaultServer")
				Expect(os.Mkdir(defaultServerPath, 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(defaultServerPath, "server.xml"), []byte{}, 0644)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				ok, err := serverBuildSource.Detect()
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			it("detects successfully if there is a server named testServer", func() {
				testServerPath := filepath.Join(serversPath, "testServer")
				Expect(os.Mkdir(testServerPath, 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(testServerPath, "server.xml"), []byte{}, 0644)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				ok, err := serverBuildSource.Detect()
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			it("detects successfully when server name is set", func() {
				testServerPath := filepath.Join(serversPath, "testServer")
				Expect(os.Mkdir(testServerPath, 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(testServerPath, "server.xml"), []byte{}, 0644)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "testServer", bard.NewLogger(ioutil.Discard))
				ok, err := serverBuildSource.Detect()
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			it("detects successfully when server name is set and there are multiple servers", func() {
				testServerPath := filepath.Join(serversPath, "testServer")
				Expect(os.Mkdir(testServerPath, 0755)).To(Succeed())
				Expect(os.Mkdir(testServerPath+"-other", 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(testServerPath, "server.xml"), []byte{}, 0644)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "testServer", bard.NewLogger(ioutil.Discard))
				ok, err := serverBuildSource.Detect()
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			it("fails to detect when server name is not set and there are multiple servers", func() {
				testServerPath := filepath.Join(serversPath, "testServer")
				Expect(os.Mkdir(testServerPath, 0755)).To(Succeed())
				Expect(os.Mkdir(testServerPath+"-other", 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(testServerPath, "server.xml"), []byte{}, 0644)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				_, err := serverBuildSource.Detect()
				Expect(err).To(HaveOccurred())
			})

			it("fails to detect if there are no servers", func() {
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				_, err := serverBuildSource.Detect()
				Expect(err).To(HaveOccurred())
			})

			it("validates enterprise archive is provided in apps", func() {
				appPath := filepath.Join(serversPath, "testServer", "apps", "test.ear")
				Expect(os.MkdirAll(appPath, 0755)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				ok, err := serverBuildSource.ValidateApp()
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			it("validates enterprise archive is provided in dropins", func() {
				appPath := filepath.Join(serversPath, "testServer", "dropins", "test.ear")
				Expect(os.MkdirAll(appPath, 0755)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				ok, err := serverBuildSource.ValidateApp()
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			it("validates web archive is provided in apps", func() {
				appPath := filepath.Join(serversPath, "testServer", "apps", "test.war")
				Expect(os.MkdirAll(appPath, 0755)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				ok, err := serverBuildSource.ValidateApp()
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			it("validates web archive is provided in dropins", func() {
				appPath := filepath.Join(serversPath, "testServer", "dropins", "test.war")
				Expect(os.MkdirAll(appPath, 0755)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				ok, err := serverBuildSource.ValidateApp()
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			it("does not validate when an app is not provided", func() {
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				_, err := serverBuildSource.ValidateApp()
				Expect(err).To(HaveOccurred())
			})
		})

		when("usr is provided", func() {
			var serversPath string

			it.Before(func() {
				serversPath = filepath.Join(testPath, "usr", "servers")
				Expect(os.MkdirAll(serversPath, 0755)).To(Succeed())
			})

			it.After(func() {
				Expect(os.RemoveAll(serversPath)).To(Succeed())
			})

			it("detects successfully if there is a server named defaultServer", func() {
				defaultServerPath := filepath.Join(serversPath, "defaultServer")
				Expect(os.Mkdir(defaultServerPath, 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(defaultServerPath, "server.xml"), []byte{}, 0644)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				ok, err := serverBuildSource.Detect()
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			it("detects successfully if there is a server named testServer", func() {
				testServerPath := filepath.Join(serversPath, "testServer")
				Expect(os.Mkdir(testServerPath, 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(testServerPath, "server.xml"), []byte{}, 0644)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				ok, err := serverBuildSource.Detect()
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			it("detects successfully when server name is set", func() {
				testServerPath := filepath.Join(serversPath, "testServer")
				Expect(os.Mkdir(testServerPath, 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(testServerPath, "server.xml"), []byte{}, 0644)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "testServer", bard.NewLogger(ioutil.Discard))
				ok, err := serverBuildSource.Detect()
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			it("detects successfully when server name is set and there are multiple servers", func() {
				testServerPath := filepath.Join(serversPath, "testServer")
				Expect(os.Mkdir(testServerPath, 0755)).To(Succeed())
				Expect(os.Mkdir(testServerPath+"-other", 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(testServerPath, "server.xml"), []byte{}, 0644)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "testServer", bard.NewLogger(ioutil.Discard))
				ok, err := serverBuildSource.Detect()
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			it("fails to detect when server name is not set and there are multiple servers", func() {
				testServerPath := filepath.Join(serversPath, "testServer")
				Expect(os.Mkdir(testServerPath, 0755)).To(Succeed())
				Expect(os.Mkdir(testServerPath+"-other", 0755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(testServerPath, "server.xml"), []byte{}, 0644)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				_, err := serverBuildSource.Detect()
				Expect(err).To(HaveOccurred())
			})

			it("fails to detect if there are no servers", func() {
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				_, err := serverBuildSource.Detect()
				Expect(err).To(HaveOccurred())
			})

			it("validates enterprise archive is provided in dropins", func() {
				appPath := filepath.Join(serversPath, "testServer", "dropins", "test.ear")
				Expect(os.MkdirAll(appPath, 0755)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				ok, err := serverBuildSource.ValidateApp()
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			it("validates web archive is provided in apps", func() {
				appPath := filepath.Join(serversPath, "testServer", "apps", "test.war")
				Expect(os.MkdirAll(appPath, 0755)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				ok, err := serverBuildSource.ValidateApp()
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			it("validates web archive is provided in dropins", func() {
				appPath := filepath.Join(serversPath, "testServer", "dropins", "test.war")
				Expect(os.MkdirAll(appPath, 0755)).To(Succeed())
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				ok, err := serverBuildSource.ValidateApp()
				Expect(err).ToNot(HaveOccurred())
				Expect(ok).To(BeTrue())
			})

			it("does not validate when an app is not provided", func() {
				serverBuildSource := core.NewServerBuildSource(testPath, "", bard.NewLogger(ioutil.Discard))
				_, err := serverBuildSource.ValidateApp()
				Expect(err).To(HaveOccurred())
			})
		})
	})
}
