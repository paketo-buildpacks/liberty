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
	"fmt"
	"github.com/paketo-buildpacks/libpak/effect"
	"io"
	"strings"
)

func DetectJVMName(executor effect.Executor) (string, error) {
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()

	resultCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "java.vm.name") {
				if strings.Contains(line, "OpenJ9") {
					resultCh <- "OpenJ9"
				} else if strings.Contains(line, "OpenJDK") {
					resultCh <- "OpenJDK"
				} else {
					resultCh <- ""
				}
				return
			}
		}
		resultCh <- ""
	}()

	if err := executor.Execute(effect.Execution{
		Command: "java",
		Args:    []string{"-XshowSettings:properties", "-version"},
		Stderr:  pw,
	}); err != nil {
		return "", fmt.Errorf("unable to detect JVM name")
	}

	pw.Close()
	return <-resultCh, nil
}
