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
	"io"
	"io/ioutil"
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

// FileExists returns true if the path exists.
func FileExists(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

// DirExists returns true if the path exists and is a directory.
func DirExists(path string) (bool, error) {
	if stat, err := os.Stat(path); err == nil {
		return stat.IsDir(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

// CopyDir copies a directory and all of its contents from the source path to the destination.
func CopyDir(src, dest string) error {
	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		fileInfo, err := os.Stat(srcPath)
		if err != nil {
			return err
		}

		switch fileInfo.Mode() & os.ModeType {
		case os.ModeDir:
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return err
			}
			if err := CopyDir(srcPath, destPath); err != nil {
				return err
			}
		default:
			if err := CopyFile(srcPath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// CopyFile copies a file from the source path to the destination path.
func CopyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	defer srcFile.Close()
	if err != nil {
		return err
	}

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
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
