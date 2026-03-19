package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---- upsert collector -------------------------------------------------

// collectingHandler accumulates upserted nodes and edges for assertions.
type collectingHandler struct {
	nodes []*types.Node
	edges []*types.Edge
}

func (h *collectingHandler) HandleUpsertNode(node *types.Node) error {
	h.nodes = append(h.nodes, node)
	return nil
}

func (h *collectingHandler) HandleUpsertEdge(edge *types.Edge) error {
	h.edges = append(h.edges, edge)
	return nil
}

// ---- helpers ----------------------------------------------------------

// mockPluginEntry builds a PluginEntry that runs the named shell script from the
// testdata directory.
func mockPluginEntry(t *testing.T, scriptName string) *PluginEntry {
	t.Helper()

	testdataDir, err := filepath.Abs("testdata/mock-sync-plugin")
	if err != nil {
		t.Fatal(err)
	}

	manifest := &types.PluginManifest{
		Name:           "test",
		Version:        "0.1.0",
		Executable:     scriptName,
		ExecutableType: types.ExecutableTypeScript,
		Capabilities:   []types.PluginCapability{types.PluginCapabilitySync},
	}

	scriptPath := filepath.Join(testdataDir, scriptName)

	// Determine the shell interpreter.
	shell := "sh"
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts not supported on Windows")
	}
	shellPath, err := lookPath(shell)
	if err != nil {
		// Fall back to a well-known location.
		shellPath = "/bin/sh"
	}

	return &PluginEntry{
		Manifest:   manifest,
		ExecPath:   shellPath,
		ScriptPath: scriptPath,
	}
}

// writeTempScript writes a shell script to a temp file and returns its path.
func writeTempScript(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "wyrd-test-plugin-*.sh")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Chmod(0o755); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

// inlineEntry builds a PluginEntry using a temporary inline script.
func inlineEntry(t *testing.T, scriptContent string) *PluginEntry {
	t.Helper()
	scriptPath := writeTempScript(t, scriptContent)

	manifest := &types.PluginManifest{
		Name:           "inline-test",
		Version:        "0.1.0",
		Executable:     filepath.Base(scriptPath),
		ExecutableType: types.ExecutableTypeScript,
		Capabilities:   []types.PluginCapability{types.PluginCapabilitySync},
	}

	shellPath := "/bin/sh"
	if p, err := lookPath("sh"); err == nil {
		shellPath = p
	}

	return &PluginEntry{
		Manifest:   manifest,
		ExecPath:   shellPath,
		ScriptPath: scriptPath,
	}
}

// ---- tests ------------------------------------------------------------

func TestRunSync_UpsertNodeFromScript(t *testing.T) {
	entry := mockPluginEntry(t, "mock-sync-plugin")
	handler := &collectingHandler{}
	runner := NewProtocolRunner(entry, 10*time.Second)

	if err := runner.RunSync(map[string]interface{}{}, handler); err != nil {
		t.Fatalf("RunSync() returned error: %v", err)
	}

	if len(handler.nodes) == 0 {
		t.Fatal("expected at least one upserted node")
	}

	// Verify that the node has a deterministic ID derived from source.
	node := handler.nodes[0]
	expected := DeterministicNodeID("mock", "item-1")
	if node.ID != expected {
		t.Errorf("expected node ID %q, got %q", expected, node.ID)
	}
	if node.Source == nil {
		t.Error("expected synced node to carry source information")
	}
}

func TestRunSync_UpsertEdgeFromScript(t *testing.T) {
	entry := mockPluginEntry(t, "mock-sync-plugin")
	handler := &collectingHandler{}
	runner := NewProtocolRunner(entry, 10*time.Second)

	if err := runner.RunSync(map[string]interface{}{}, handler); err != nil {
		t.Fatalf("RunSync() returned error: %v", err)
	}

	if len(handler.edges) == 0 {
		t.Fatal("expected at least one upserted edge")
	}
}

func TestRunAction_UpsertNodeFromScript(t *testing.T) {
	entry := mockPluginEntry(t, "mock-sync-plugin")
	handler := &collectingHandler{}
	runner := NewProtocolRunner(entry, 10*time.Second)

	node := &types.Node{
		ID:    "test-node-id",
		Body:  "Test node",
		Types: []string{"task"},
	}

	if err := runner.RunAction(node, map[string]interface{}{}, handler); err != nil {
		t.Fatalf("RunAction() returned error: %v", err)
	}

	if len(handler.nodes) == 0 {
		t.Fatal("expected at least one upserted node from action")
	}
}

func TestRunSync_MalformedJSONLineSkipped(t *testing.T) {
	script := `#!/bin/sh
read -r line
printf 'THIS IS NOT JSON\n'
printf '{"op":"upsert_node","node":{"body":"Good node","types":["task"],"source":{"type":"mock","id":"ok-1"}}}\n'
printf '{"op":"done","summary":{"created":1,"updated":0,"errors":0}}\n'
`
	entry := inlineEntry(t, script)
	handler := &collectingHandler{}
	runner := NewProtocolRunner(entry, 10*time.Second)

	// Should not return an error — malformed lines are skipped.
	if err := runner.RunSync(map[string]interface{}{}, handler); err != nil {
		t.Fatalf("RunSync() should skip malformed lines, got error: %v", err)
	}

	if len(handler.nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(handler.nodes))
	}
}

func TestRunSync_Timeout(t *testing.T) {
	script := `#!/bin/sh
read -r line
sleep 10
printf '{"op":"done","summary":{"created":0,"updated":0,"errors":0}}\n'
`
	entry := inlineEntry(t, script)
	handler := &collectingHandler{}
	runner := NewProtocolRunner(entry, 200*time.Millisecond)

	err := runner.RunSync(map[string]interface{}{}, handler)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	pluginErr, ok := err.(*types.PluginError)
	if !ok {
		t.Fatalf("expected *types.PluginError, got %T: %v", err, err)
	}
	if pluginErr.Plugin != "inline-test" {
		t.Errorf("expected plugin name %q, got %q", "inline-test", pluginErr.Plugin)
	}
}

func TestRunSync_ProcessCrash(t *testing.T) {
	script := `#!/bin/sh
read -r line
exit 1
`
	entry := inlineEntry(t, script)
	handler := &collectingHandler{}
	runner := NewProtocolRunner(entry, 5*time.Second)

	// A crash (immediate exit without done) should result in EOF — treated as
	// implicit completion, so no error is expected.
	_ = runner.RunSync(map[string]interface{}{}, handler)
}

func TestWireToNode_DeterministicID(t *testing.T) {
	wn := &wireNode{
		Body:  "Synced node",
		Types: []string{"task"},
		Source: &wireSource{
			Type: "github",
			ID:   "issue-99",
		},
	}

	node1 := wireToNode(wn)
	node2 := wireToNode(wn)

	if node1.ID != node2.ID {
		t.Errorf("expected deterministic ID, got %q and %q", node1.ID, node2.ID)
	}
	expected := DeterministicNodeID("github", "issue-99")
	if node1.ID != expected {
		t.Errorf("expected ID %q, got %q", expected, node1.ID)
	}
}

func TestWireToNode_FallbackUUID(t *testing.T) {
	wn := &wireNode{
		Body:  "Local node",
		Types: []string{"note"},
	}
	node := wireToNode(wn)
	if node.ID == "" {
		t.Error("expected a generated UUID for nodes without source")
	}
}

func TestOutboundMessage_Serialisation(t *testing.T) {
	msg := outboundMessage{
		Op:     "sync",
		Config: map[string]interface{}{"key": "value"},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}

	var roundtrip outboundMessage
	if err := json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if roundtrip.Op != "sync" {
		t.Errorf("expected op %q, got %q", "sync", roundtrip.Op)
	}
}
