package plugin

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// tokenPattern matches {{property.path}} interpolation tokens.
var tokenPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

// ShellExecutor is the built-in wyrd-plugin-shell executor.
// It reads a command template from the manifest's config_schema.command field,
// interpolates {{property.path}} tokens from the target node's properties, and
// runs the result via the system shell.
type ShellExecutor struct {
	manifest *types.PluginManifest
	config   map[string]interface{}
	timeout  time.Duration
}

// NewShellExecutor creates a ShellExecutor for the given manifest and config.
// If timeout is zero the default (30 s) is used.
func NewShellExecutor(manifest *types.PluginManifest, config map[string]interface{}, timeout time.Duration) *ShellExecutor {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &ShellExecutor{
		manifest: manifest,
		config:   config,
		timeout:  timeout,
	}
}

// Execute interpolates the command template from the manifest's config_schema
// using the given node's properties and runs the result.
func (s *ShellExecutor) Execute(node *types.Node) error {
	template, err := s.commandTemplate()
	if err != nil {
		return err
	}

	interpolated, err := interpolateTemplate(template, node)
	if err != nil {
		return &types.PluginError{
			Plugin:  s.manifest.Name,
			Message: fmt.Sprintf("template interpolation: %v", err),
		}
	}

	return runShellCommand(interpolated, s.timeout)
}

// commandTemplate extracts the command template from config_schema.command.
func (s *ShellExecutor) commandTemplate() (string, error) {
	schema := s.manifest.ConfigSchema
	if schema == nil {
		return "", &types.PluginError{
			Plugin:  s.manifest.Name,
			Message: "config_schema is missing",
		}
	}
	raw, ok := schema["command"]
	if !ok {
		return "", &types.PluginError{
			Plugin:  s.manifest.Name,
			Message: "config_schema.command is missing",
		}
	}
	template, ok := raw.(string)
	if !ok {
		return "", &types.PluginError{
			Plugin:  s.manifest.Name,
			Message: "config_schema.command must be a string",
		}
	}
	return template, nil
}

// interpolateTemplate replaces every {{property.path}} token in the template
// string with the corresponding value from the node's properties.
// Returns an error if any referenced property is absent (never silently empty).
func interpolateTemplate(template string, node *types.Node) (string, error) {
	var interpolationErr error

	result := tokenPattern.ReplaceAllStringFunc(template, func(match string) string {
		if interpolationErr != nil {
			return ""
		}
		// Extract the path inside {{ }}.
		inner := strings.TrimSpace(match[2 : len(match)-2])
		val, err := resolvePropertyPath(node.Properties, inner)
		if err != nil {
			interpolationErr = fmt.Errorf("property path %q not found: %w", inner, err)
			return ""
		}
		return fmt.Sprintf("%v", val)
	})

	if interpolationErr != nil {
		return "", interpolationErr
	}
	return result, nil
}

// resolvePropertyPath resolves a dot-separated property path against a property map.
// For example, "github.repo" looks up props["github"]["repo"].
func resolvePropertyPath(props map[string]interface{}, path string) (interface{}, error) {
	parts := strings.SplitN(path, ".", 2)
	key := parts[0]

	val, ok := props[key]
	if !ok {
		return nil, fmt.Errorf("property %q not found", key)
	}

	if len(parts) == 1 {
		return val, nil
	}

	// Recurse into nested map.
	nested, ok := val.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("property %q is not a map — cannot traverse path %q", key, path)
	}
	return resolvePropertyPath(nested, parts[1])
}

// runShellCommand executes a command string via the system shell within the
// given timeout. The command is passed to sh -c to support pipes, redirections,
// and other shell features.
func runShellCommand(command string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("shell command timed out after %s", timeout)
		}
		return fmt.Errorf("shell command failed: %w\noutput: %s", err, string(output))
	}
	return nil
}
