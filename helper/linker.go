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

package helper

import (
	"fmt"
	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/liberty/internal/util"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/bindings"
	"github.com/paketo-buildpacks/libpak/sherpa"
	"os"
	"path/filepath"
)

type FileLinker struct {
	Bindings        libcnb.Bindings
	Logger          bard.Logger
	RuntimeRootPath string
	ServerRootPath  string
}

func (f FileLinker) Execute() (map[string]string, error) {
	var err error

	if err = f.Configure(); err != nil {
		return map[string]string{}, fmt.Errorf("unable to configure\n%w", err)
	}
	return map[string]string{}, nil
}

func (f FileLinker) Configure() error {
	serverRootPath, err := getServerPath()
	if err != nil {
		return fmt.Errorf("unable to get server root path\n%w", err)
	}

	b, hasBindings, err := bindings.ResolveOne(f.Bindings, bindings.OfType("liberty"))
	if err != nil {
		return fmt.Errorf("unable to resolve bindings\n%w", err)
	}

	if !hasBindings {
		return nil
	}

	if bindingXML, ok := b.SecretFilePath("server.xml"); ok {
		if err = util.DeleteAndLinkPath(bindingXML, filepath.Join(serverRootPath, "server.xml")); err != nil {
			return fmt.Errorf("unable to replace server.xml\n%w", err)
		}
	}

	if bootstrapProperties, ok := b.SecretFilePath("bootstrap.properties"); ok {
		existingBSP := filepath.Join(serverRootPath, "bootstrap.properties")
		if err = util.DeleteAndLinkPath(bootstrapProperties, existingBSP); err != nil {
			return fmt.Errorf("unable to replace bootstrap.properties\n%w", err)
		}
	}

	return nil
}

func getServerPath() (string, error) {
	usrPath, err := sherpa.GetEnvRequired("WLP_USER_DIR")
	if err != nil {
		return "", err
	}

	_, err = os.Stat(usrPath)
	if err != nil && os.IsNotExist(err) {
		return "", fmt.Errorf("unable to find '%s', folder does not exist", usrPath)
	} else if err != nil {
		return "", fmt.Errorf("unable to check %s\n%w", usrPath, err)
	}

	serverName, err := sherpa.GetEnvRequired("BPI_LIBERTY_SERVER_NAME")
	if err != nil {
		return "", err
	}

	return filepath.Join(usrPath, "servers", serverName), nil
}
