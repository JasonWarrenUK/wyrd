package cli_test

import (
	"os/exec"
)

// commandFromPath returns an exec.Cmd that runs the given path directly.
func commandFromPath(path string) *exec.Cmd {
	return exec.Command(path)
}
