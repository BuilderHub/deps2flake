// Package flake is the template layer for generated Nix flakes.
package flake

import (
	"fmt"
	"os"
)

// EnsureWritable verifies that path may be written under the overwrite policy.
func EnsureWritable(path string, force bool) error {
	if path == "" {
		return fmt.Errorf("flake path is required")
	}
	if force {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists; use --force to overwrite it", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking existing flake: %w", err)
	}
	return nil
}
