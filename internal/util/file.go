package util

import (
	"fmt"
	"os"
)

// LinkPath removes the destination path (if it exists) and creates a symlink from the source to the destination path.
func LinkPath(from, to string) error {
	if _, err := os.Stat(from); err != nil {
		return fmt.Errorf("unable to find file '%s'\n%w", from, err)
	}
	if err := os.Remove(to); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to delete original file '%s'\n%w", from, err)
	}
	if err := os.Symlink(from, to); err != nil {
		return fmt.Errorf("unable to symlink file from '%s' to '%s'\n%w", from, to, err)
	}
	return nil
}
