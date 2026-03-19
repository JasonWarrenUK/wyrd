package sync

import (
	"os"
)

// readFileSafe reads a file and returns its content.
// Returns an error if the file does not exist or cannot be read.
func readFileSafe(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// writeFile writes content to path, creating or truncating the file.
func writeFile(path string, content []byte) error {
	return os.WriteFile(path, content, 0o644)
}

// openFileAppend opens path for appending, creating it if necessary.
func openFileAppend(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}
