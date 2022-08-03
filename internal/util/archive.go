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

package util

import (
	"fmt"
	"os"
	"strings"

	"github.com/paketo-buildpacks/libpak/crush"
)

func Extract(artifact *os.File, destination string, stripComponents int) error {
	artifactName := artifact.Name()
	var err error
	if strings.HasSuffix(artifactName, ".tar.gz") || strings.HasSuffix(artifactName, ".tgz") {
		err = crush.ExtractTarGz(artifact, destination, stripComponents)
	} else if strings.HasSuffix(artifactName, ".zip") ||
		strings.HasSuffix(artifactName, ".jar") ||
		strings.HasSuffix(artifactName, ".war") ||
		strings.HasSuffix(artifactName, ".ear") {
		err = crush.ExtractZip(artifact, destination, stripComponents)
	} else if strings.HasSuffix(artifactName, ".tar") {
		err = crush.ExtractTar(artifact, destination, stripComponents)
	} else if strings.HasSuffix(artifactName, ".tar.bz2") {
		err = crush.ExtractTarBz2(artifact, destination, stripComponents)
	} else if strings.HasSuffix(artifactName, ".tar.xz") {
		err = crush.ExtractTarXz(artifact, destination, stripComponents)
	} else {
		return fmt.Errorf("unable to read archive of type: %s", artifactName)
	}

	return err
}
