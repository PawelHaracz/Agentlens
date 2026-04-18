package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// agentSearch calls GET /api/v1/catalog and returns matching catalog entries.
// Project scope comes from ctx (AccessibleProjectIDs) — never from tool args.
func agentSearch(lb LoopbackFunc) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (any, error) {
		in, err := parseAgentSearch(args)
		if err != nil {
			return nil, err
		}
		body, status, err := lb(ctx, http.MethodGet, "/api/v1/catalog", buildSearchQuery(in))
		if err != nil {
			return nil, fmt.Errorf("agent_search loopback: %w", err)
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("agent_search: catalog returned %d", status)
		}
		return wrapContent(body), nil
	}
}

// agentGet calls GET /api/v1/catalog/{id} and returns the single entry.
func agentGet(lb LoopbackFunc) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (any, error) {
		id, err := parseID(args, "id")
		if err != nil {
			return nil, err
		}
		body, status, err := lb(ctx, http.MethodGet, "/api/v1/catalog/"+id, "")
		if err != nil {
			return nil, fmt.Errorf("agent_get loopback: %w", err)
		}
		if status == http.StatusNotFound {
			return nil, fmt.Errorf("agent not found: %s", id)
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("agent_get: catalog returned %d", status)
		}
		return wrapContent(body), nil
	}
}

// capabilitiesList calls GET /api/v1/capabilities?agent_id={id}.
func capabilitiesList(lb LoopbackFunc) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (any, error) {
		agentID, err := parseID(args, "agent_id")
		if err != nil {
			return nil, err
		}
		body, status, err := lb(ctx, http.MethodGet, "/api/v1/capabilities", "agent_id="+agentID)
		if err != nil {
			return nil, fmt.Errorf("capabilities_list loopback: %w", err)
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("capabilities_list: returned %d", status)
		}
		return wrapContent(body), nil
	}
}

// agentCard fetches the raw agent card for an agent.
func agentCard(lb LoopbackFunc) ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (any, error) {
		agentID, err := parseID(args, "agent_id")
		if err != nil {
			return nil, err
		}
		body, status, err := lb(ctx, http.MethodGet, "/api/v1/catalog/"+agentID+"/card", "")
		if err != nil {
			return nil, fmt.Errorf("agent_card loopback: %w", err)
		}
		if status == http.StatusNotFound {
			return nil, fmt.Errorf("agent card not found: %s", agentID)
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("agent_card: returned %d", status)
		}
		return wrapContent(body), nil
	}
}
