package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"go-harness/pkg/tracing"

	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// ToolHandler defines the execution signature for a registered tool.
type ToolHandler func(ctx context.Context, args json.RawMessage) (any, error)

// ToolDefinition defines metadata and schema for an agent tool.
type ToolDefinition struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Category    string             `json:"category"` // "builtin", "mcp", "custom", "memory", "flow"
	Enabled     bool               `json:"enabled"`
	Parameters  jsonschema.Definition `json:"parameters"`
	Handler     ToolHandler        `json:"-"`
}

// Registry manages the set of available agent tools.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*ToolDefinition
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]*ToolDefinition),
	}
}

// Register adds or updates a tool definition in the registry.
func (r *Registry) Register(def ToolDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[def.Name] = &def
}

// Unregister removes a tool definition by name.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (*ToolDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, exists := r.tools[name]
	if !exists {
		return nil, false
	}
	cpy := *t
	return &cpy, true
}

// SetEnabled toggles the enabled state of a tool.
func (r *Registry) SetEnabled(name string, enabled bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, exists := r.tools[name]
	if !exists {
		return false
	}
	t.Enabled = enabled
	return true
}

// List returns a list of all registered tools.
func (r *Registry) List() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		list = append(list, *t)
	}
	return list
}

// Execute invokes a registered tool with tracing.
func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (any, error) {
	r.mu.RLock()
	t, exists := r.tools[name]
	r.mu.RUnlock()

	if !exists {
		err := fmt.Errorf("tool not found: %s", name)
		tracing.TraceToolCall(ctx, name, string(args), nil, err)
		return nil, err
	}

	if !t.Enabled {
		err := fmt.Errorf("tool '%s' is currently disabled", name)
		tracing.TraceToolCall(ctx, name, string(args), nil, err)
		return nil, err
	}

	if t.Handler == nil {
		err := fmt.Errorf("tool '%s' has no executable handler", name)
		tracing.TraceToolCall(ctx, name, string(args), nil, err)
		return nil, err
	}

	result, err := t.Handler(ctx, args)
	tracing.TraceToolCall(ctx, name, string(args), result, err)
	return result, err
}

// ToOpenAITools converts enabled tools into the OpenAI Tool format.
func (r *Registry) ToOpenAITools() []openai.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []openai.Tool
	for _, t := range r.tools {
		if !t.Enabled {
			continue
		}
		result = append(result, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return result
}
