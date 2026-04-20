// Package tools provides the MCP ToolRegistry and the four discovery tools.
//
// v2 translator path (spec §6.5): when the OpenAPI-to-MCP translator ships,
// it will emit ToolEntry values with the same {Name, Description, InputSchema,
// Handler} shape and call Register() for each generated tool. The registry
// interface is deliberately narrow so the generated tools are a drop-in.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// LoopbackFunc is a function that dispatches an HTTP-like call through the
// in-process chi router and returns the response body + status code.
// Constructed by internal/api.BuildLoopbackFunc and injected at startup.
// type alias avoids plugins importing internal/api (arch-go boundary).
type LoopbackFunc func(ctx context.Context, method, path, query string) (body []byte, status int, err error)

// ToolHandler is the function signature for a registered tool.
type ToolHandler func(ctx context.Context, args json.RawMessage) (any, error)

// ToolDescriptor describes a tool for tools/list responses (spec §6.1).
type ToolDescriptor struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

// Registry holds registered MCP tools and dispatches calls.
// It implements the mcpserver.ToolRegistry interface.
type Registry struct {
	descriptors []ToolDescriptor
	handlers    map[string]ToolHandler
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{handlers: make(map[string]ToolHandler)}
}

// Register adds a tool. Panics on duplicate name (caught at startup).
func (r *Registry) Register(desc ToolDescriptor, handler ToolHandler) {
	if _, dup := r.handlers[desc.Name]; dup {
		panic("mcpserver/tools: duplicate tool name: " + desc.Name)
	}
	r.descriptors = append(r.descriptors, desc)
	r.handlers[desc.Name] = handler
}

// Call dispatches a tool call. Returns CodeInternalError-style error on miss.
func (r *Registry) Call(ctx context.Context, name string, args json.RawMessage) (any, error) {
	h, ok := r.handlers[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return h(ctx, args)
}

// List returns all registered tool descriptors.
func (r *Registry) List() []ToolDescriptor {
	if r.descriptors == nil {
		return []ToolDescriptor{}
	}
	return r.descriptors
}
