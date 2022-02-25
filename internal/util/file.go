package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// DeleteAndLinkPath removes the destination path (if it exists) and creates a symlink from the source to the
// destination path.
func DeleteAndLinkPath(src, dest string) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("unable to find src path '%s'\n%w", src, err)
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
