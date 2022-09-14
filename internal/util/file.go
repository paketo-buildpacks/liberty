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
	"github.com/paketo-buildpacks/libpak/sherpa"
	"os"
	"path/filepath"
)

// DeleteAndLinkPath removes the destination path (if it exists) and creates a symlink from the source to the
// destination path.
func DeleteAndLinkPath(src, dest string) error {
	if _, err := os.Stat(src); err != nil {
		return err
	}
	if err := os.RemoveAll(dest); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to delete original destination path '%s'\n%w", dest, err)
	}
	if err := os.Symlink(src, dest); err != nil {
		return fmt.Errorf("unable to symlink file from '%s' to '%s'\n%w", src, dest, err)
	}
	return nil
}

func GetFiles(root string, pattern string) ([]string, error) {
	var files []string
	if exists, err := sherpa.DirExists(root); err != nil {
		return nil, err
	} else if !exists {
		return nil, nil
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err != nil {
			return err
		}
		if matched {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
