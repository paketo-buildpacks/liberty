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

package helper

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/bindings"
)

type FileLinker struct {
	Bindings libcnb.Bindings
	Logger   bard.Logger
}

func (f FileLinker) Execute() (map[string]string, error) {
	var err error
	appDir, ok := os.LookupEnv("BPI_OL_DROPIN_DIR")
	if !ok {
		appDir = "/workspace"
	}

	layerDir, ok := os.LookupEnv("BPI_OL_RUNTIME_ROOT")
	if !ok {
		layerDir = "/layers/paketo-buildpacks_open-liberty/open-liberty-runtime"
	}

	b, ok, err := bindings.ResolveOne(f.Bindings, bindings.OfType("open-liberty"))
	if err != nil {
		return nil, fmt.Errorf("error resolving bindings: %w", err)
	}

	if ok {
		bindingXML, ok := b.SecretFilePath("server.xml")
		if ok {
			serverXML := filepath.Join(layerDir, "usr", "servers", "defaultServer", "server.xml")
			if _, err = os.Stat(serverXML); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("error checking for server.xml: %w", err)
			}

			if err = os.Remove(serverXML); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("error deleting original server.xml: %w", err)
			}

			if err = os.Symlink(bindingXML, serverXML); err != nil {
				return nil, fmt.Errorf("error linking server.xml: %w", err)
			}
		}

		bootstrapProperties, ok := b.SecretFilePath("bootstrap.properties")
		if ok {
			existingBSP := filepath.Join(layerDir, "usr", "servers", "defaultServer", "bootstrap.properties")
			if _, err = os.Stat(existingBSP); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("error checking for bootstrap.properties: %w", err)
			}

			if err = os.Remove(existingBSP); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("error removing existing bootstrap.properties: %w", err)
			}

			if err = os.Symlink(bootstrapProperties, existingBSP); err != nil {
				return nil, fmt.Errorf("error linking bootstrap.properties: %w", err)
			}
		}
	}

	linkPath := filepath.Join(layerDir, "usr", "servers", "defaultServer", "dropins", f.getLinkName(appDir))

	os.Remove(linkPath) // we don't care if this succeeds or fails necessarily, we just want to try to remove anything in the way of the relinking

	return nil, os.Symlink(appDir, linkPath)
}

func (f FileLinker) getLinkName(appDir string) string {
	name := os.Getenv("BPL_OPENLIBERTY_APP_NAME")
	if name == "" {
		name = filepath.Base(appDir)
	}

	// first, let's check to see if it's an EAR
	_, err := os.Stat(filepath.Join(appDir, "META-INF", "application.xml"))
	if err == nil {
		return name + ".ear"
	}

	// now, we can check if it's a war
	_, err = os.Stat(filepath.Join(appDir, "WEB-INF", "web.xml"))
	if err == nil {
		return name + ".war"
	}

	// at this point, we don't know what it is, so just return the name
	return name
}
