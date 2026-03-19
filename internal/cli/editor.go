package cli

import (
	"fmt"
	"os"
	"os/exec"
)

// OpenEditor opens filePath in the user's preferred editor.
// It reads $EDITOR from the environment, falling back to "vi".
func OpenEditor(filePath string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	cmd := exec.Command(editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor %q exited with error: %w", editor, err)
	}
	return nil
}
