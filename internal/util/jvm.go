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
	"bufio"
	"bytes"
	"fmt"
	"github.com/paketo-buildpacks/libpak/effect"
	"strings"
)

func DetectJVMName(executor effect.Executor) (string, error) {
	buf := &bytes.Buffer{}

	if err := executor.Execute(effect.Execution{
		Command: "java",
		Args:    []string{"-XshowSettings:properties", "-version"},
		Stdout:  buf,
	}); err != nil {
		return "", fmt.Errorf("unable to detect JVM name\n%w", err)
	}

	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "java.vm.name") {
			if strings.Contains(line, "OpenJ9") {
				return "OpenJ9", nil
			} else if strings.Contains(line, "OpenJDK") {
				return "OpenJDK", nil
			}
			break
		}
	}
	return "", nil
}
