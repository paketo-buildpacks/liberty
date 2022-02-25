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