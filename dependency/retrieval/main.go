// Copyright 2018-2026 the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/paketo-buildpacks/libdependency/buildpack_config"
	"github.com/paketo-buildpacks/libdependency/retrieve"
	"github.com/paketo-buildpacks/libdependency/upstream"
	"github.com/paketo-buildpacks/libdependency/versionology"
	"github.com/paketo-buildpacks/packit/v2/cargo"
)

const (
	mavenRepository = "https://repo1.maven.org/maven2"

	openLibertyGroupID = "io.openliberty"
	openLibertyPURL    = "openliberty"
	openLibertyLicense = "https://raw.githubusercontent.com/OpenLiberty/open-liberty/integration/LICENSE"
	openLibertySource  = "https://github.com/OpenLiberty/open-liberty/archive/refs/tags/gm-%s.tar.gz"

	webSphereGroupID = "com.ibm.websphere.appserver.runtime"
	webSpherePURL    = "websphere-liberty"
	webSphereLicense = "https://public.dhe.ibm.com/ibmdl/export/pub/software/websphere/wasdev/downloads/wlp/23.0.0.3/lafiles/runtime/en.html"
)

// distribution maps a dependency id to the Maven artifact that provides it and
// the metadata used to generate its PURL, CPE, and license fields.
type distribution struct {
	id          string
	groupID     string
	artifact    string
	name        string
	purlName    string
	license     string
	licenseType string
}

var distributions = []distribution{
	{"open-liberty-runtime-full", openLibertyGroupID, "openliberty-runtime", "Open Liberty (All Features)", openLibertyPURL, openLibertyLicense, "EPL-2.0"},
	{"open-liberty-runtime-jakartaee11", openLibertyGroupID, "openliberty-jakartaee11", "Open Liberty (Jakarta EE11)", openLibertyPURL, openLibertyLicense, "EPL-2.0"},
	{"open-liberty-runtime-javaee8", openLibertyGroupID, "openliberty-javaee8", "Open Liberty (Java EE8)", openLibertyPURL, openLibertyLicense, "EPL-2.0"},
	{"open-liberty-runtime-webProfile11", openLibertyGroupID, "openliberty-webProfile11", "Open Liberty (Web Profile 11)", openLibertyPURL, openLibertyLicense, "EPL-2.0"},
	{"open-liberty-runtime-webProfile8", openLibertyGroupID, "openliberty-webProfile8", "Open Liberty (Web Profile 8)", openLibertyPURL, openLibertyLicense, "EPL-2.0"},
	{"open-liberty-runtime-microProfile4", openLibertyGroupID, "openliberty-microProfile4", "Open Liberty (Micro Profile 4)", openLibertyPURL, openLibertyLicense, "EPL-2.0"},
	{"open-liberty-runtime-microProfile7", openLibertyGroupID, "openliberty-microProfile7", "Open Liberty (Micro Profile 7)", openLibertyPURL, openLibertyLicense, "EPL-2.0"},
	{"open-liberty-runtime-kernel", openLibertyGroupID, "openliberty-kernel", "Open Liberty (Kernel)", openLibertyPURL, openLibertyLicense, "EPL-2.0"},
	{"websphere-liberty-runtime-kernel", webSphereGroupID, "wlp-kernel", "WebSphere Liberty (Kernel)", webSpherePURL, webSphereLicense, "Proprietary"},
	{"websphere-liberty-runtime-jakartaee11", webSphereGroupID, "wlp-jakartaee11", "WebSphere Liberty (Jakarta EE11)", webSpherePURL, webSphereLicense, "Proprietary"},
	{"websphere-liberty-runtime-javaee8", webSphereGroupID, "wlp-javaee8", "WebSphere Liberty (Java EE8)", webSpherePURL, webSphereLicense, "Proprietary"},
	{"websphere-liberty-runtime-javaee7", webSphereGroupID, "wlp-javaee7", "WebSphere Liberty (Java EE7)", webSpherePURL, webSphereLicense, "Proprietary"},
	{"websphere-liberty-runtime-webProfile11", webSphereGroupID, "wlp-webProfile11", "WebSphere Liberty (Web Profile 11)", webSpherePURL, webSphereLicense, "Proprietary"},
	{"websphere-liberty-runtime-webProfile8", webSphereGroupID, "wlp-webProfile8", "WebSphere Liberty (Web Profile 8)", webSpherePURL, webSphereLicense, "Proprietary"},
	{"websphere-liberty-runtime-webProfile7", webSphereGroupID, "wlp-webProfile7", "WebSphere Liberty (Web Profile 7)", webSpherePURL, webSphereLicense, "Proprietary"},
}

type generator struct {
	id               string
	getAllVersions   retrieve.GetAllVersionsFunc
	generateMetadata retrieve.GenerateMetadataFunc
}

// mavenVersion adapts the four-part Maven version (e.g. 26.0.0.9) to the
// three-part buildpack version (e.g. 26.0.9). The third component of the Maven
// version is always 0, so dropping it preserves ordering while producing the
// version written to buildpack.toml.
type mavenVersion struct {
	version *semver.Version
	raw     string
}

func (v mavenVersion) Version() *semver.Version {
	return v.version
}

type mavenMetadata struct {
	Versioning struct {
		Versions struct {
			Version []string `xml:"version"`
		} `xml:"versions"`
	} `xml:"versioning"`
}

func main() {
	buildpackTomlPath, output := retrieve.FetchArgs()

	config, err := buildpack_config.ParseBuildpackToml(buildpackTomlPath)
	if err != nil {
		panic(err)
	}

	var dependencies []versionology.Dependency
	for _, g := range generators() {
		newVersions, err := retrieve.GetNewVersionsForId(g.id, config, g.getAllVersions)
		if err != nil {
			panic(err)
		}

		dependencies = append(dependencies, retrieve.GenerateAllMetadata(newVersions, g.generateMetadata)...)
	}

	metadataJSON, err := json.Marshal(dependencies)
	if err != nil {
		panic(fmt.Errorf("unable to marshal metadata json\n%w", err))
	}

	if err := os.WriteFile(output, metadataJSON, os.ModePerm); err != nil {
		panic(fmt.Errorf("cannot write to %s: %w", output, err))
	}

	fmt.Printf("Wrote metadata to %s\n", output)
}

func generators() []generator {
	generators := make([]generator, 0, len(distributions))
	for _, d := range distributions {
		generators = append(generators, mavenGenerator(d))
	}
	return generators
}

func mavenGenerator(d distribution) generator {
	return generator{
		id: d.id,
		getAllVersions: func() (versionology.VersionFetcherArray, error) {
			return getAllMavenVersions(d)
		},
		generateMetadata: func(versionFetcher versionology.VersionFetcher) ([]versionology.Dependency, error) {
			return generateMavenMetadata(d, versionFetcher)
		},
	}
}

func getAllMavenVersions(d distribution) (versionology.VersionFetcherArray, error) {
	metadataURL := fmt.Sprintf("%s/%s/%s/maven-metadata.xml", mavenRepository, strings.ReplaceAll(d.groupID, ".", "/"), d.artifact)

	body, err := getBody(metadataURL)
	if err != nil {
		return nil, err
	}

	var metadata mavenMetadata
	if err := xml.Unmarshal([]byte(body), &metadata); err != nil {
		return nil, fmt.Errorf("unable to parse %s\n%w", metadataURL, err)
	}

	var versions versionology.VersionFetcherArray
	for _, raw := range metadata.Versioning.Versions.Version {
		bpv, err := buildpackVersion(raw)
		if err != nil {
			fmt.Printf("Skipping %s: %s\n", raw, err)
			continue
		}

		version, err := semver.NewVersion(bpv)
		if err != nil {
			fmt.Printf("Skipping %s: unable to parse version\n", raw)
			continue
		}

		versions = append(versions, mavenVersion{version: version, raw: raw})
	}

	return versions, nil
}

func generateMavenMetadata(d distribution, versionFetcher versionology.VersionFetcher) ([]versionology.Dependency, error) {
	version, ok := versionFetcher.(mavenVersion)
	if !ok {
		return nil, fmt.Errorf("unexpected version type %T", versionFetcher)
	}

	raw := version.raw
	versionString := version.version.String()

	base := fmt.Sprintf("%s/%s/%s/%s", mavenRepository, strings.ReplaceAll(d.groupID, ".", "/"), d.artifact, raw)
	uri := fmt.Sprintf("%s/%s-%s.zip", base, d.artifact, raw)
	checksum, err := upstream.GetSHA256OfRemoteFile(uri)
	if err != nil {
		return nil, fmt.Errorf("unable to checksum %s\n%w", uri, err)
	}

	dependency := cargo.ConfigMetadataDependency{
		Checksum: fmt.Sprintf("sha256:%s", checksum),
		CPE:      cpe(d, raw),
		ID:       d.id,
		Licenses: []interface{}{
			map[string]string{
				"type": d.licenseType,
				"uri":  d.license,
			},
		},
		Name:    d.name,
		PURL:    retrieve.GeneratePURL(d.purlName, raw, checksum, uri),
		Stacks:  []string{"*"},
		URI:     uri,
		Version: versionString,
	}

	// Open Liberty publishes source tarballs alongside its release tags, but the
	// WebSphere artifacts are proprietary and have no public source.
	if d.groupID == openLibertyGroupID {
		source := fmt.Sprintf(openLibertySource, raw)
		sourceChecksum, err := upstream.GetSHA256OfRemoteFile(source)
		if err != nil {
			return nil, fmt.Errorf("unable to checksum %s\n%w", source, err)
		}
		dependency.Source = source
		dependency.SourceChecksum = fmt.Sprintf("sha256:%s", sourceChecksum)
	}

	return versionology.NewDependencyArray(dependency, "")
}

// buildpackVersion converts a four-part Maven version (e.g. 26.0.0.9) into the
// three-part version used in buildpack.toml (e.g. 26.0.9). Versions with any
// other number of components are returned unchanged so they still parse.
func buildpackVersion(raw string) (string, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 4 {
		return raw, nil
	}

	for _, part := range parts {
		if _, err := strconv.Atoi(part); err != nil {
			return "", fmt.Errorf("non-numeric version component in %q", raw)
		}
	}

	return strings.Join([]string{parts[0], parts[1], parts[3]}, "."), nil
}

func cpe(d distribution, version string) string {
	if d.groupID == openLibertyGroupID {
		return fmt.Sprintf("cpe:2.3:a:ibm:open_liberty:%s:*:*:*:*:*:*:*", version)
	}
	return fmt.Sprintf("cpe:2.3:a:ibm:websphere_application_server:%s:*:*:*:liberty:*:*:*", version)
}

func getBody(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("unable to fetch %s\n%w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unable to fetch %s: status code %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("unable to read %s\n%w", url, err)
	}

	return string(body), nil
}
