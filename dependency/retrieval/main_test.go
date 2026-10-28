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

import "testing"

func TestMavenPURL(t *testing.T) {
	tests := []struct {
		name    string
		d       distribution
		version string
		want    string
	}{
		{
			name:    "open liberty",
			d:       distribution{groupID: "io.openliberty", artifact: "openliberty-runtime"},
			version: "26.0.0.10",
			want:    "pkg:maven/io.openliberty/openliberty-runtime@26.0.0.10",
		},
		{
			name:    "websphere liberty",
			d:       distribution{groupID: "com.ibm.websphere.appserver.runtime", artifact: "wlp-kernel"},
			version: "26.0.0.10",
			want:    "pkg:maven/com.ibm.websphere.appserver.runtime/wlp-kernel@26.0.0.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mavenPURL(tt.d, tt.version); got != tt.want {
				t.Errorf("mavenPURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
