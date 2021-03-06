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

package liberty

import (
	"fmt"

	"github.com/buildpacks/libcnb"
	"github.com/paketo-buildpacks/liberty/internal/core"
	"github.com/paketo-buildpacks/libpak"
	"github.com/paketo-buildpacks/libpak/bard"
)

const (
	PlanEntryLiberty               = "liberty"
	PlanEntryJRE                   = "jre"
	PlanEntryJDK                   = "jdk"
	PlanEntryJVMApplicationPackage = "jvm-application-package"
	PlanEntryJavaAppServer         = "java-app-server"
	PlanEntrySyft                  = "syft"
)

type Detect struct {
	Logger bard.Logger
}

func (d Detect) Detect(context libcnb.DetectContext) (libcnb.DetectResult, error) {
	cr, err := libpak.NewConfigurationResolver(context.Buildpack, nil)
	if err != nil {
		return libcnb.DetectResult{}, fmt.Errorf("unable to create configuration resolver\n%w", err)
	}

	serverName, _ := cr.Resolve("BP_LIBERTY_SERVER_NAME")
	requestedAppServer, _ := cr.Resolve("BP_JAVA_APP_SERVER")
	serverBuildSrc := core.NewServerBuildSource(context.Application.Path, serverName, d.Logger)
	appBuildSrc := core.NewAppBuildSource(context.Application.Path, requestedAppServer, d.Logger)

	buildSources := []core.BuildSource{
		serverBuildSrc,
		appBuildSrc,
	}

	var detectedBuildSrc core.BuildSource

	for _, buildSrc := range buildSources {
		d.Logger.Debugf("Checking build source '%s'", buildSrc.Name())
		ok, err := buildSrc.Detect()
		if err != nil {
			return libcnb.DetectResult{},
				fmt.Errorf("unable to detect build source '%s'\n%w", buildSrc.Name(), err)
		}
		if ok {
			detectedBuildSrc = buildSrc
			break
		}
	}

	if detectedBuildSrc == nil {
		return libcnb.DetectResult{Pass: false}, nil
	}

	d.Logger.Debugf("Detected build source '%s'", detectedBuildSrc.Name())

	result := libcnb.DetectResult{
		Pass: true,
		Plans: []libcnb.BuildPlan{
			{
				Provides: []libcnb.BuildPlanProvide{
					{Name: PlanEntryLiberty},
					{Name: PlanEntryJavaAppServer},
				},
				Requires: []libcnb.BuildPlanRequire{
					{Name: PlanEntryJRE, Metadata: map[string]interface{}{"launch": true}},
					{Name: PlanEntryJDK},
					{Name: PlanEntryJavaAppServer},
					{Name: PlanEntryJVMApplicationPackage},
					{Name: PlanEntryLiberty},
					{Name: PlanEntrySyft},
				},
			},
		},
	}

	validApp, err := detectedBuildSrc.ValidateApp()
	if err != nil {
		return libcnb.DetectResult{},
			fmt.Errorf("unable to check if app was provided for build source '%s'\n%w", detectedBuildSrc.Name(), err)
	}

	if !validApp {
		d.Logger.Debugf("No applications provided in build source '%s'", detectedBuildSrc.Name())
		return result, nil
	}

	result.Plans[0].Provides = append(result.Plans[0].Provides,
		libcnb.BuildPlanProvide{
			Name: PlanEntryJVMApplicationPackage,
		},
	)

	return result, nil
}
