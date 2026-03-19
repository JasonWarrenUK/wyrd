package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"time"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// defaultTimeout is used when the caller does not specify a timeout.
const defaultTimeout = 30 * time.Second

// UpsertHandler receives upsert messages produced by a plugin during sync or action.
type UpsertHandler interface {
	HandleUpsertNode(node *types.Node) error
	HandleUpsertEdge(edge *types.Edge) error
}

// ProtocolRunner manages the lifecycle of a single plugin invocation and
// handles the JSON-lines communication protocol.
type ProtocolRunner struct {
	entry   *PluginEntry
	timeout time.Duration
}

// NewProtocolRunner creates a ProtocolRunner for the given plugin entry.
// If timeout is zero the default (30 s) is used.
func NewProtocolRunner(entry *PluginEntry, timeout time.Duration) *ProtocolRunner {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &ProtocolRunner{entry: entry, timeout: timeout}
}

// ---- wire types -------------------------------------------------------

// outboundMessage is sent from Wyrd to the plugin.
type outboundMessage struct {
	Op     string                 `json:"op"`
	Config map[string]interface{} `json:"config,omitempty"`
	Node   *wireNode              `json:"node,omitempty"`
}

// inboundMessage is received from the plugin.
type inboundMessage struct {
	Op      string                 `json:"op"`
	Node    *wireNode              `json:"node,omitempty"`
	Edge    *wireEdge              `json:"edge,omitempty"`
	Summary *syncSummary           `json:"summary,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Props   map[string]interface{} `json:"-"`
}

// syncSummary carries the terminal sync statistics.
type syncSummary struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	Errors  int `json:"errors"`
}

// wireNode is the JSON representation of a node on the wire.
type wireNode struct {
	ID         string                 `json:"id,omitempty"`
	Body       string                 `json:"body"`
	Types      []string               `json:"types"`
	Source     *wireSource            `json:"source,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// wireEdge is the JSON representation of an edge on the wire.
type wireEdge struct {
	ID         string                 `json:"id,omitempty"`
	Type       string                 `json:"type"`
	From       string                 `json:"from"`
	To         string                 `json:"to"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// wireSource mirrors types.Source for JSON serialisation on the wire.
type wireSource struct {
	Type string `json:"type"`
	Repo string `json:"repo,omitempty"`
	ID   string `json:"id"`
	URL  string `json:"url,omitempty"`
}

// ---- RunSync ----------------------------------------------------------

// RunSync sends a sync message to the plugin and processes its responses until
// a "done" message is received or the timeout expires.
func (r *ProtocolRunner) RunSync(config map[string]interface{}, handler UpsertHandler) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	cmd := r.buildCommand(ctx)
	stdin, stdout, err := pipeCommand(cmd)
	if err != nil {
		return &types.PluginError{Plugin: r.entry.Manifest.Name, Message: fmt.Sprintf("start plugin: %v", err)}
	}

	// Send the sync trigger message.
	msg := outboundMessage{Op: "sync", Config: config}
	if err := writeMessage(stdin, msg); err != nil {
		return &types.PluginError{Plugin: r.entry.Manifest.Name, Message: fmt.Sprintf("write sync message: %v", err)}
	}
	stdin.Close()

	return r.readResponses(ctx, stdout, cmd, handler)
}

// RunAction sends an action message to the plugin for the given node and
// processes its responses.
func (r *ProtocolRunner) RunAction(node *types.Node, config map[string]interface{}, handler UpsertHandler) error {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	cmd := r.buildCommand(ctx)
	stdin, stdout, err := pipeCommand(cmd)
	if err != nil {
		return &types.PluginError{Plugin: r.entry.Manifest.Name, Message: fmt.Sprintf("start plugin: %v", err)}
	}

	msg := outboundMessage{
		Op:     "action",
		Config: config,
		Node:   nodeToWire(node),
	}
	if err := writeMessage(stdin, msg); err != nil {
		return &types.PluginError{Plugin: r.entry.Manifest.Name, Message: fmt.Sprintf("write action message: %v", err)}
	}
	stdin.Close()

	return r.readResponses(ctx, stdout, cmd, handler)
}

// readResponses reads JSON-lines from the plugin until a "done" line arrives,
// an error occurs, or the context deadline is exceeded.
func (r *ProtocolRunner) readResponses(ctx context.Context, stdout io.Reader, cmd *exec.Cmd, handler UpsertHandler) error {
	scanner := bufio.NewScanner(stdout)
	done := make(chan error, 1)

	go func() {
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var msg inboundMessage
			if err := json.Unmarshal(line, &msg); err != nil {
				// Malformed JSON-line: log and continue.
				log.Printf("plugin %s: malformed JSON-line — skipping: %v", r.entry.Manifest.Name, err)
				continue
			}

			switch msg.Op {
			case "upsert_node":
				if msg.Node == nil {
					log.Printf("plugin %s: upsert_node with nil node — skipping", r.entry.Manifest.Name)
					continue
				}
				node := wireToNode(msg.Node)
				if err := handler.HandleUpsertNode(node); err != nil {
					log.Printf("plugin %s: upsert_node failed: %v", r.entry.Manifest.Name, err)
				}

			case "upsert_edge":
				if msg.Edge == nil {
					log.Printf("plugin %s: upsert_edge with nil edge — skipping", r.entry.Manifest.Name)
					continue
				}
				edge := wireToEdge(msg.Edge)
				if err := handler.HandleUpsertEdge(edge); err != nil {
					log.Printf("plugin %s: upsert_edge failed: %v", r.entry.Manifest.Name, err)
				}

			case "done":
				if msg.Summary != nil {
					log.Printf("plugin %s: sync complete — created:%d updated:%d errors:%d",
						r.entry.Manifest.Name,
						msg.Summary.Created,
						msg.Summary.Updated,
						msg.Summary.Errors,
					)
				}
				done <- nil
				return

			case "error":
				done <- &types.PluginError{Plugin: r.entry.Manifest.Name, Message: msg.Error}
				return

			default:
				log.Printf("plugin %s: unknown op %q — skipping", r.entry.Manifest.Name, msg.Op)
			}
		}

		if err := scanner.Err(); err != nil {
			done <- &types.PluginError{Plugin: r.entry.Manifest.Name, Message: fmt.Sprintf("read stdout: %v", err)}
			return
		}

		// EOF without a "done" message — treat as implicit completion.
		done <- nil
	}()

	select {
	case err := <-done:
		_ = cmd.Wait()
		return err
	case <-ctx.Done():
		// Timeout or cancellation: kill the process.
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		return &types.PluginError{
			Plugin:  r.entry.Manifest.Name,
			Message: fmt.Sprintf("plugin timed out after %s", r.timeout),
		}
	}
}

// ---- helpers ----------------------------------------------------------

// buildCommand constructs the exec.Cmd for the plugin entry.
func (r *ProtocolRunner) buildCommand(ctx context.Context) *exec.Cmd {
	if r.entry.ScriptPath != "" {
		// Script: interpreter <script>
		return exec.CommandContext(ctx, r.entry.ExecPath, r.entry.ScriptPath)
	}
	return exec.CommandContext(ctx, r.entry.ExecPath)
}

// pipeCommand starts the command and returns connected stdin/stdout pipes.
func pipeCommand(cmd *exec.Cmd) (io.WriteCloser, io.Reader, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return stdin, stdout, nil
}

// writeMessage serialises msg as a single JSON line to w.
func writeMessage(w io.Writer, msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// nodeToWire converts a types.Node to its wire representation.
func nodeToWire(n *types.Node) *wireNode {
	wn := &wireNode{
		ID:         n.ID,
		Body:       n.Body,
		Types:      n.Types,
		Properties: n.Properties,
	}
	if n.Source != nil {
		wn.Source = &wireSource{
			Type: n.Source.Type,
			Repo: n.Source.Repo,
			ID:   n.Source.ID,
			URL:  n.Source.URL,
		}
	}
	return wn
}

// wireToNode converts a wireNode into a types.Node, deriving a deterministic ID
// when source information is present.
func wireToNode(wn *wireNode) *types.Node {
	node := &types.Node{
		Body:       wn.Body,
		Types:      wn.Types,
		Properties: wn.Properties,
		Created:    time.Now(),
		Modified:   time.Now(),
	}

	if wn.Source != nil {
		node.Source = &types.Source{
			Type:       wn.Source.Type,
			Repo:       wn.Source.Repo,
			ID:         wn.Source.ID,
			URL:        wn.Source.URL,
			LastSynced: time.Now(),
		}
		// Derive a deterministic UUID v5 from the source type and ID.
		node.ID = DeterministicNodeID(wn.Source.Type, wn.Source.ID)
	} else if wn.ID != "" {
		node.ID = wn.ID
	} else {
		node.ID = uuid.NewString()
	}

	return node
}

// wireToEdge converts a wireEdge into a types.Edge.
func wireToEdge(we *wireEdge) *types.Edge {
	edge := &types.Edge{
		Type:       we.Type,
		From:       we.From,
		To:         we.To,
		Properties: we.Properties,
		Created:    time.Now(),
	}
	if we.ID != "" {
		edge.ID = we.ID
	} else {
		edge.ID = uuid.NewString()
	}
	return edge
}
