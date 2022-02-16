package server

import (
	"fmt"
	"github.com/paketo-buildpacks/open-liberty/internal/util"
	"io/ioutil"
	"path/filepath"
	"strings"
)

type LibertyServer struct {
	InstallRoot string
	ServerName  string
}

func (s LibertyServer) GetServerConfigPath() string {
	return filepath.Join(s.InstallRoot, "usr", "servers", s.ServerName, "server.xml")
}

// SetUserDirectory sets the server's user directory to the specified directory.
func (s LibertyServer) SetUserDirectory(path string) error {
	// Copy the configDropins directory to the new user directory. This is needed by Liberty runtimes provided in the
	// stack run image
	configDropinsDir := filepath.Join(s.InstallRoot, "usr", "servers", "defaultServer", "configDropins")
	if configDropinsFound, err := util.DirExists(configDropinsDir); err != nil {
		return fmt.Errorf("unable to read configDropins directory\n%w", err)
	} else if configDropinsFound {
		newConfigDropinsDir := filepath.Join(path, "servers", s.ServerName, "configDropins")
		if err := util.CopyDir(configDropinsDir, newConfigDropinsDir); err != nil {
			return fmt.Errorf("unable to copy configDropins to new user directory\n%w", err)
		}
	}
	if err := util.DeleteAndLinkPath(path, filepath.Join(s.InstallRoot, "usr")); err != nil {
		return fmt.Errorf("unable to set new user directory\n%w", err)
	}
	return nil
}

// HasInstalledApps checks the directories `<server-path>/apps` and `<server-path>/dropins` for any web or enterprise
// archives. Returns true if it finds at least one compiled artifact.
func (s LibertyServer) HasInstalledApps() (bool, error) {
	serverPath := filepath.Join(s.InstallRoot, "usr", "servers", s.ServerName)
	if hasApps, err := dirHasCompiledArtifacts(filepath.Join(serverPath, "apps")); err != nil {
		return false, fmt.Errorf("unable to check apps directory for app archives\n%w", err)
	} else if hasApps {
		return true, nil
	}

	if hasDropins, err := dirHasCompiledArtifacts(filepath.Join(serverPath, "dropins")); err != nil {
		return false, fmt.Errorf("unable to check dropins directory for app archives\n%w", err)
	} else if hasDropins {
		return true, nil
	}

	return false, nil
}

// dirHasCompiledArtifacts checks if the given directory has any web or enterprise archives.
func dirHasCompiledArtifacts(path string) (bool, error) {
	if exists, err := util.DirExists(path); err != nil {
		return false, err
	} else if !exists {
		return false, nil
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return false, err
	}
	for _, file := range files {
		if name := file.Name(); strings.HasSuffix(name, ".war") || strings.HasSuffix(name, ".ear") {
			return true, nil
		}
	}
	return false, nil
}
