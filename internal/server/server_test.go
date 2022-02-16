package server_test

import (
	"github.com/paketo-buildpacks/open-liberty/internal/server"
	"github.com/sclevine/spec"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func testServer(t *testing.T, when spec.G, it spec.S) {
	var (
		Expect   = NewWithT(t).Expect
		svr      server.LibertyServer
		testPath string
	)

	it.Before(func() {
		var err error
		testPath, err = ioutil.TempDir("", "server")
		Expect(err).NotTo(HaveOccurred())

		// EvalSymlinks on macOS resolves the temporary directory too so do that here or checking the symlinks will fail
		testPath, err = filepath.EvalSymlinks(testPath)
		Expect(err).NotTo(HaveOccurred())

		svr = server.LibertyServer{
			InstallRoot: filepath.Join(testPath, "wlp"),
			ServerName:  "defaultServer",
		}
		Expect(os.MkdirAll(filepath.Join(svr.InstallRoot, "usr", "servers", "defaultServer", "apps"), 0755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(svr.InstallRoot, "usr", "servers", "defaultServer", "dropins"), 0755)).To(Succeed())
	})

	it.After(func() {
		Expect(os.RemoveAll(testPath)).To(Succeed())
		Expect(os.RemoveAll(filepath.Join(svr.InstallRoot, "usr", "servers", "defaultServer", "apps"))).To(Succeed())
		Expect(os.RemoveAll(filepath.Join(svr.InstallRoot, "usr", "servers", "defaultServer", "dropins"))).To(Succeed())
	})

	when("changing the server user directory", func() {
		it("works with no configDropins in original user directory", func() {
			// Create new user directory
			newUserDir := filepath.Join(testPath, "new-user-dir")
			newServerDir := filepath.Join(newUserDir, "servers", "defaultServer")
			Expect(os.MkdirAll(newServerDir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(newServerDir, "server.xml"), []byte{}, 0644)).To(Succeed())

			Expect(svr.SetUserDirectory(newUserDir)).To(Succeed())
			newConfigPath, err := filepath.EvalSymlinks(svr.GetServerConfigPath())
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
			configDropinsDir := filepath.Join(svr.InstallRoot, "usr", "servers", svr.ServerName, "configDropins", "overrides")
			Expect(os.MkdirAll(configDropinsDir, 0755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(configDropinsDir, "test-config.xml"), []byte{}, 0644)).To(Succeed())

			Expect(svr.SetUserDirectory(newUserDir)).To(Succeed())
			Expect(filepath.Join(configDropinsDir, "test-config.xml")).To(BeARegularFile())

			Expect(os.RemoveAll(filepath.Join(svr.InstallRoot, "usr"))).To(Succeed())
			Expect(os.RemoveAll(newUserDir)).To(Succeed())
		})
	})

	when("checking if a server has installed apps", func() {
		it("finds war in apps directory", func() {
			appPath := filepath.Join(svr.InstallRoot, "usr", "servers", "defaultServer", "apps", "test.war")
			Expect(os.MkdirAll(appPath, 0755)).To(Succeed())
			Expect(svr.HasInstalledApps()).To(BeTrue())
			Expect(os.RemoveAll(appPath)).To(Succeed())
		})

		it("finds ear in apps directory", func() {
			appPath := filepath.Join(svr.InstallRoot, "usr", "servers", "defaultServer", "apps", "test.ear")
			Expect(os.MkdirAll(appPath, 0755)).To(Succeed())
			Expect(svr.HasInstalledApps()).To(BeTrue())
			Expect(os.RemoveAll(appPath)).To(Succeed())
		})

		it("finds war in dropins directory", func() {
			appPath := filepath.Join(svr.InstallRoot, "usr", "servers", "defaultServer", "dropins", "test.war")
			Expect(os.MkdirAll(appPath, 0755)).To(Succeed())
			Expect(svr.HasInstalledApps()).To(BeTrue())
			Expect(os.RemoveAll(appPath)).To(Succeed())
		})

		it("finds ear in dropins directory", func() {
			appPath := filepath.Join(svr.InstallRoot, "usr", "servers", "defaultServer", "dropins", "test.ear")
			Expect(os.MkdirAll(appPath, 0755)).To(Succeed())
			Expect(svr.HasInstalledApps()).To(BeTrue())
			Expect(os.RemoveAll(appPath)).To(Succeed())
		})
	})
}
