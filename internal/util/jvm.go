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
	"errors"
	"fmt"
	"github.com/paketo-buildpacks/libpak/bard"
	"github.com/paketo-buildpacks/libpak/effect"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// DetectJVMName returns the value for java.vm.name.
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

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("unable to read cache ratio\n%w", err)
	}

	return "", nil
}

type SharedClassCache struct {
	Name string
	Path string

	Executor effect.Executor
	Logger   bard.Logger
}

type SharedClassCacheOptions struct {
	Enabled       bool
	SizeMB        int
	NumIterations int
	Trim          bool
}

func (scc SharedClassCache) GetFillRatio() (float64, error) {
	sccOption := fmt.Sprintf("-Xshareclasses:name=%s,cacheDir=%s,printTopLayerStats", scc.Name, scc.Path)
	args := []string{sccOption, "-version"}
	buf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	err := scc.Executor.Execute(effect.Execution{
		Command: "java",
		Args:    args,
		Stdout:  buf,
		Stderr:  stderrBuf,
	})
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() != 1 {
		return 0.0, fmt.Errorf("unable to get cache stats\n%w\n%s", err, stderrBuf.String())
	}

	r, err := regexp.Compile(`Cache is (\d+)% full`)
	if err != nil {
		return 0.0, err
	}

	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := scanner.Text()
		scc.Logger.Debugf("%s\n", line)
		if matches := r.FindStringSubmatch(line); matches != nil {
			v, err := strconv.ParseInt(matches[1], 10, 64)
			if err != nil {
				return 0.0, err
			}
			return float64(v) / 100, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return 0.0, fmt.Errorf("unable to read cache ratio\n%w", err)
	}

	return 0.0, fmt.Errorf("unable to find cache fill ratio")
}

func (scc SharedClassCache) CreateLayer(size int) error {
	scc.Logger.Bodyf("Creating SCC layer with size %dm", size)
	return scc.execSccCommand("createLayer,groupAccess", fmt.Sprintf("-Xscmx%dm", size))
}

func (scc SharedClassCache) Resize(size int) error {
	fillRatio, err := scc.GetFillRatio()
	if err != nil {
		return err
	}
	newSize := int(float64(size)*fillRatio + 0.5)
	scc.Logger.Bodyf("Resizing cache from %dm to %dm", size, newSize)
	if err := scc.Delete(); err != nil {
		return err
	}
	if err := scc.CreateLayer(newSize); err != nil {
		return fmt.Errorf("unable to create resized layer\n%w", err)
	}
	return nil
}

func (scc SharedClassCache) Delete() error {
	err := scc.execSccCommand("destroy")
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return nil
	}
	if err != nil {
		return fmt.Errorf("unable to destroy cache\n%w", err)
	}
	return nil
}

func (scc SharedClassCache) execSccCommand(command string, options ...string) error {
	sccOption := fmt.Sprintf("-Xshareclasses:name=%s,cacheDir=%s,%s", scc.Name, scc.Path, command)
	writer := io.Discard
	if scc.Logger.IsDebugEnabled() {
		sccOption = fmt.Sprintf("%s,verbose", sccOption)
		writer = scc.Logger.InfoWriter()
	}
	args := []string{sccOption}
	if options != nil {
		args = append(args, options...)
	}
	return scc.Executor.Execute(effect.Execution{
		Command: "java",
		Args:    append(args, "-version"),
		Stdout:  writer,
		Stderr:  writer,
	})
}
