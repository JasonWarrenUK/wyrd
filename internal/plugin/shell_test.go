package plugin

import (
	"strings"
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---- helpers ----------------------------------------------------------

func makeNode(props map[string]interface{}) *types.Node {
	return &types.Node{
		ID:         "test-id",
		Body:       "Test node",
		Types:      []string{"task"},
		Properties: props,
	}
}

func makeManifest(command string) *types.PluginManifest {
	return &types.PluginManifest{
		Name:           "shell-test",
		Version:        "1.0.0",
		Executable:     "wyrd-plugin-shell",
		ExecutableType: types.ExecutableTypeBinary,
		Capabilities:   []types.PluginCapability{types.PluginCapabilityAction},
		ConfigSchema: map[string]interface{}{
			"command": command,
		},
	}
}

// ---- interpolation tests ----------------------------------------------

func TestInterpolateTemplate_SimpleProperty(t *testing.T) {
	node := makeNode(map[string]interface{}{"title": "My Task"})
	result, err := interpolateTemplate("echo {{title}}", node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "echo My Task" {
		t.Errorf("expected %q, got %q", "echo My Task", result)
	}
}

func TestInterpolateTemplate_NestedProperty(t *testing.T) {
	node := makeNode(map[string]interface{}{
		"github": map[string]interface{}{
			"repo": "jasonwarrenuk/wyrd",
		},
	})
	result, err := interpolateTemplate("open https://github.com/{{github.repo}}", node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "open https://github.com/jasonwarrenuk/wyrd" {
		t.Errorf("expected %q, got %q", "open https://github.com/jasonwarrenuk/wyrd", result)
	}
}

func TestInterpolateTemplate_MissingProperty_ReturnsError(t *testing.T) {
	node := makeNode(map[string]interface{}{"title": "My Task"})
	_, err := interpolateTemplate("echo {{url}}", node)
	if err == nil {
		t.Fatal("expected error for missing property, got nil")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("expected error to mention %q, got: %v", "url", err)
	}
}

func TestInterpolateTemplate_MissingNestedProperty_ReturnsError(t *testing.T) {
	node := makeNode(map[string]interface{}{
		"github": map[string]interface{}{
			"repo": "wyrd",
		},
	})
	_, err := interpolateTemplate("{{github.issue_number}}", node)
	if err == nil {
		t.Fatal("expected error for missing nested property, got nil")
	}
}

func TestInterpolateTemplate_NonMapParent_ReturnsError(t *testing.T) {
	// Trying to traverse into a non-map value should fail.
	node := makeNode(map[string]interface{}{
		"title": "flat string",
	})
	_, err := interpolateTemplate("{{title.sub}}", node)
	if err == nil {
		t.Fatal("expected error when traversing into a non-map property")
	}
}

func TestInterpolateTemplate_MultipleTokens(t *testing.T) {
	node := makeNode(map[string]interface{}{
		"first": "John",
		"last":  "Doe",
	})
	result, err := interpolateTemplate("echo {{first}} {{last}}", node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "echo John Doe" {
		t.Errorf("expected %q, got %q", "echo John Doe", result)
	}
}

func TestInterpolateTemplate_NoTokens(t *testing.T) {
	node := makeNode(map[string]interface{}{})
	result, err := interpolateTemplate("echo hello", node)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "echo hello" {
		t.Errorf("expected %q, got %q", "echo hello", result)
	}
}

// ---- executor tests ---------------------------------------------------

func TestShellExecutor_MissingConfigSchema_ReturnsError(t *testing.T) {
	manifest := &types.PluginManifest{
		Name:         "shell-test",
		ConfigSchema: nil,
	}
	executor := NewShellExecutor(manifest, nil, 5*time.Second)
	node := makeNode(map[string]interface{}{})
	err := executor.Execute(node)
	if err == nil {
		t.Fatal("expected error for missing config_schema")
	}
}

func TestShellExecutor_MissingCommand_ReturnsError(t *testing.T) {
	manifest := &types.PluginManifest{
		Name:         "shell-test",
		ConfigSchema: map[string]interface{}{},
	}
	executor := NewShellExecutor(manifest, nil, 5*time.Second)
	node := makeNode(map[string]interface{}{})
	err := executor.Execute(node)
	if err == nil {
		t.Fatal("expected error for missing config_schema.command")
	}
}

func TestShellExecutor_Execute_Success(t *testing.T) {
	manifest := makeManifest("echo {{title}}")
	node := makeNode(map[string]interface{}{"title": "hello"})
	executor := NewShellExecutor(manifest, nil, 5*time.Second)
	if err := executor.Execute(node); err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
}

func TestShellExecutor_Execute_MissingProperty(t *testing.T) {
	manifest := makeManifest("echo {{url}}")
	node := makeNode(map[string]interface{}{"title": "no url here"})
	executor := NewShellExecutor(manifest, nil, 5*time.Second)
	err := executor.Execute(node)
	if err == nil {
		t.Fatal("expected error for missing property in command template")
	}
}

func TestShellExecutor_Execute_Timeout(t *testing.T) {
	manifest := makeManifest("sleep 10")
	node := makeNode(map[string]interface{}{})
	executor := NewShellExecutor(manifest, nil, 100*time.Millisecond)
	err := executor.Execute(node)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestResolvePropertyPath_TopLevel(t *testing.T) {
	props := map[string]interface{}{"key": "value"}
	val, err := resolvePropertyPath(props, "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "value" {
		t.Errorf("expected %q, got %q", "value", val)
	}
}

func TestResolvePropertyPath_Nested(t *testing.T) {
	props := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner": 42,
		},
	}
	val, err := resolvePropertyPath(props, "outer.inner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Errorf("expected 42, got %v", val)
	}
}

func TestResolvePropertyPath_Missing(t *testing.T) {
	props := map[string]interface{}{"a": "1"}
	_, err := resolvePropertyPath(props, "b")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}
