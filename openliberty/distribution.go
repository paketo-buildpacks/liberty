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

package openliberty

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/libjvm/count"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/crush"
	"github.com/paketo-buildpacks/libpak/effect"
)

type Distribution struct {
	ServerName       string
	ApplicationPath  string
	LayerContributor libpak.DependencyLayerContributor
	Logger           bard.Logger
}

func NewDistribution(dependency libpak.BuildpackDependency, cache libpak.DependencyCache, serverName, applicationPath string) (Distribution, libcnb.BOMEntry) {
	contributor, entry := libpak.NewDependencyLayer(dependency, cache, libcnb.LayerTypes{
		Cache:  true,
		Launch: true,
	})
	return Distribution{
		ServerName:       serverName,
		ApplicationPath:  applicationPath,
		LayerContributor: contributor,
	}, entry
}

func (d Distribution) Contribute(layer libcnb.Layer) (libcnb.Layer, error) {
	d.LayerContributor.Logger = d.Logger

	return d.LayerContributor.Contribute(layer, func(artifact *os.File) (libcnb.Layer, error) {
		d.Logger.Bodyf("Expanding to %s", layer.Path)
		if err := crush.ExtractZip(artifact, layer.Path, 1); err != nil {
			return libcnb.Layer{}, fmt.Errorf("unable to expand Liberty Runtime\n%w", err)
		}

		executor := effect.NewExecutor()
		if err := executor.Execute(effect.Execution{
			Command: filepath.Join(layer.Path, "bin", "server"),
			Args:    []string{"create", d.ServerName},
			Dir:     layer.Path,
			Stdout:  bard.NewWriter(d.Logger.InfoWriter(), bard.WithIndent(3)),
			Stderr:  bard.NewWriter(d.Logger.InfoWriter(), bard.WithIndent(3)),
		}); err != nil {
			return libcnb.Layer{}, fmt.Errorf("could not create default server\n%w", err)
		}

		libertyClasses, err := count.Classes(layer.Path)
		if err != nil {
			return libcnb.Layer{}, fmt.Errorf("could not count liberty classes\n%w", err)
		}

		layer.LaunchEnvironment.Default("BPL_JVM_CLASS_ADJUSTMENT", strconv.Itoa(libertyClasses))

		// these are used by the exec.d helper to successfully create the symlink to the dropin app
		layer.LaunchEnvironment.Default("BPI_OL_DROPIN_DIR", d.ApplicationPath)
		layer.LaunchEnvironment.Default("BPI_OL_RUNTIME_ROOT", layer.Path)

		// set logging to write to the console. Using `server run` instead of `server start` ensures that
		// stdout/stderr are actually written to their respective streams instead of to `console.log`
		layer.LaunchEnvironment.Default("WLP_LOGGING_MESSAGE_SOURCE", "")
		layer.LaunchEnvironment.Default("WLP_LOGGING_CONSOLE_SOURCE", "message,trace,accessLog,ffdc,audit")

		// because of a liberty design decision, we can only force things to stdout if we are logging in
		// JSON format
		layer.LaunchEnvironment.Default("WLP_LOGGING_MESSAGE_FORMAT", "JSON")
		layer.LaunchEnvironment.Default("WLP_LOGGING_CONSOLE_FORMAT", "JSON")
		layer.LaunchEnvironment.Default("WLP_LOGGING_APPS_WRITE_JSON", "true")
		layer.LaunchEnvironment.Default("WLP_LOGGING_JSON_ACCESS_LOG_FIELDS", "default")

		return layer, nil
	})
}

func (d Distribution) Name() string {
	return d.LayerContributor.LayerName()
}
