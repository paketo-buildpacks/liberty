package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/buildpacks/libcnb"
	"github.com/cheggaaa/pb/v3"
	"github.com/paketo-buildpacks/libpak"
)

var defaultBuildpack libcnb.Buildpack

var libertyLicenses = []libpak.BuildpackDependencyLicense{
	{
		Type: "EPL-1.0",
		URI:  "https://raw.githubusercontent.com/OpenLiberty/open-liberty/master/LICENSE",
	},
}

const (
	baseURL      = "https://public.dhe.ibm.com/ibmdl/export/pub/software/openliberty/runtime/release"
	infoJSON     = "info.json"
	NUM_VERSIONS = 2
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	defaultBuildpack.API = "0.6"
	defaultBuildpack.Info = libcnb.BuildpackInfo{
		ID:          "paketo-buildpacks/open-liberty",
		Name:        "Paketo Open Liberty Buildpack",
		Version:     "{{.version}}",
		Homepage:    "https://github.com/paketo-buildpacks/open-liberty",
		Description: "A Cloud Native Buildpack that provides the Open Liberty runtime",
		Keywords:    []string{"java", "javaee", "open-liberty"},
		Licenses:    []libcnb.License{{Type: "Apache-2.0", URI: "https://github.com/paketo-buildpacks/open-liberty/blob/main/LICENSE"}},
	}
	defaultBuildpack.Stacks = []libcnb.BuildpackStack{{ID: "io.buildpacks.stacks.bionic"}}

	log.Print("Fetching versions...")
	resp, err := http.Get(fmt.Sprintf("%s/%s", baseURL, infoJSON))
	if err != nil {
		log.Fatalf("error fetching open liberty versions index: %v", err)
	}
	defer resp.Body.Close()

	jsonBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("error reading versions index: %v", err)
	}

	versionsIndex := map[string][]string{}
	if err = json.Unmarshal(jsonBytes, &versionsIndex); err != nil {
		log.Fatalf("malformed versions index: %v", err)
	}

	versions, ok := versionsIndex["versions"]
	if !ok || len(versions) == 0 {
		log.Fatal("versions index missing 'versions' key or no versions found")
	}

	verSlice := sort.StringSlice(versions)
	sort.Sort(sort.Reverse(verSlice))

	allDependencies := make([]libpak.BuildpackDependency, 0, 10)

	for i := 0; i < NUM_VERSIONS && i < len(versions); i++ {
		log.Printf("Getting all dependencies for internal version %s", versions[i])
		deps, err := getAllDependenciesForVersion(versions[i])
		if err != nil {
			log.Fatalf("error getting dependencies for version %s: %v", versions[i], err)
		}
		allDependencies = append(allDependencies, deps...)
	}

	latestVersion := strings.Split(allDependencies[0].Version, "-")[0]

	defaultBuildpack.Metadata = map[string]interface{}{
		"configurations": []libpak.BuildpackConfiguration{
			{
				Name:        "BP_OPENLIBERTY_VERSION",
				Description: "Which version of the Open Liberty runtime to install. Defaults to latest supported version",
				Default:     latestVersion,
				Build:       true,
				Launch:      false,
			},
			{
				Name:        "BP_OPENLIBERTY_PROFILE",
				Description: "The Liberty profile to install. Current options are 'full', 'javaee8', 'webProfile8', 'microProfile4', and 'kernel'",
				Default:     "full",
				Build:       true,
				Launch:      false,
			},
		},
		"dependencies": allDependencies,
		"pre-package":  "scripts/build.sh",
		"include-files": []string{
			"LICENSE",
			"NOTICE",
			"README.md",
			"bin/build",
			"bin/detect",
			"bin/main",
			"bin/helper",
			"buildpack.toml",
		},
	}

	writer := os.Stdout
	if len(os.Args) > 1 && os.Args[1] != "-" {
		if writer, err = os.OpenFile(os.Args[1], os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644); err != nil {
			log.Fatalf("could not open output file %s: %v", os.Args[1], err)
		}
	}

	if err = toml.NewEncoder(writer).Encode(defaultBuildpack); err != nil {
		log.Fatalf("could not write output: %v", err)
	}
}

type versionInfo struct {
	Version         string   `json:"version"`
	FullInstall     string   `json:"driver_location"`
	ProfileInstalls []string `json:"package_locations"`
}

func getAllDependenciesForVersion(version string) ([]libpak.BuildpackDependency, error) {
	deps := make([]libpak.BuildpackDependency, 0, 5)

	versionBase := fmt.Sprintf("%s/%s", baseURL, version)
	resp, err := http.Get(fmt.Sprintf("%s/%s", versionBase, infoJSON))
	if err != nil {
		return nil, fmt.Errorf("error fetching version %s info: %w", version, err)
	}
	defer resp.Body.Close()

	jsonBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading version %s info: %w", version, err)
	}

	vi := &versionInfo{}
	if err = json.Unmarshal(jsonBytes, &vi); err != nil {
		return nil, fmt.Errorf("error parsing version %s info: %w", version, err)
	}

	// IBM does not use semver for their versioning, instead using four parts. Because
	// the buildpack.toml parser requires a valid semver, we strip out the 3rd part, as
	// all versions are currently of the format X.0.0.Y, and have been since 2018 at least
	verParts := strings.Split(vi.Version, ".")
	semver := fmt.Sprintf("%s.%s.%s", verParts[0], verParts[1], verParts[3])

	log.Printf("Getting variants for version %s", semver)

	fullURL := fmt.Sprintf("%s/%s", versionBase, vi.FullInstall)
	deps = append(deps, libpak.BuildpackDependency{
		Name:     "open-liberty-runtime",
		Version:  fmt.Sprintf("%s-full", semver),
		Stacks:   []string{"io.buildpacks.stacks.bionic"},
		Licenses: libertyLicenses,
		URI:      fullURL,
		SHA256:   getChecksum(fullURL),
	})

	for _, profile := range vi.ProfileInstalls {
		profileName := strings.Split(profile, "-")[1]
		uri := fmt.Sprintf("%s/%s", versionBase, profile)
		deps = append(deps, libpak.BuildpackDependency{
			Name:     "open-liberty-runtime",
			Version:  fmt.Sprintf("%s-%s", semver, profileName),
			Stacks:   []string{"io.buildpacks.stacks.bionic"},
			Licenses: libertyLicenses,
			URI:      uri,
			SHA256:   getChecksum(uri),
		})
	}

	return deps, nil
}

func getChecksum(url string) string {
	log.Printf("Calculating sha256 checksum for %s", url)
	hash := sha256.New()

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	progressBar := pb.New64(resp.ContentLength)
	defer progressBar.Finish()

	progressBar.SetWriter(os.Stderr)
	progressBar.Start()

	if _, err = io.Copy(hash, progressBar.NewProxyReader(resp.Body)); err != nil {
		log.Fatal(err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil))
}
