package deps

import (
	"fmt"
	"os/exec"
)

// CheckDependency verifies if a binary is available in PATH.
// Returns nil if found, error with install hint if not.
func CheckDependency(name string) error {
	_, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s not found in PATH: please install it first", name)
	}
	return nil
}

// CheckAll verifies multiple dependencies at once.
// Returns the first error encountered, or nil if all are found.
func CheckAll(deps ...string) error {
	for _, dep := range deps {
		if err := CheckDependency(dep); err != nil {
			return err
		}
	}
	return nil
}
