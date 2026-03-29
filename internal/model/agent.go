// Package model defines the core data types for AgentLens.
package model

import (
	"encoding/json"
	"time"
)

// Protocol represents the agent communication protocol.
type Protocol string

const (
	ProtocolA2A  Protocol = "a2a"
	ProtocolMCP  Protocol = "mcp"
	ProtocolA2UI Protocol = "a2ui"
)

// Status represents the health status of an agent.
type Status string

const (
	StatusHealthy  Status = "healthy"
	StatusDegraded Status = "degraded"
	StatusDown     Status = "down"
	StatusUnknown  Status = "unknown"
)

// SourceType represents how the agent was discovered.
type SourceType string

const (
	SourceK8s      SourceType = "k8s"
	SourceConfig   SourceType = "config"
	SourcePush     SourceType = "push"
	SourceUpstream SourceType = "upstream"
)

// Agent represents an AI agent registered in the catalog.
type Agent struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Protocol    Protocol        `json:"protocol"`
	Endpoint    string          `json:"endpoint"`
	Version     string          `json:"version"`
	Status      Status          `json:"status"`
	Source      SourceType      `json:"source"`
	Namespace   string          `json:"namespace,omitempty"`
	Team        string          `json:"team,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Skills      []Skill         `json:"skills,omitempty"`
	RawCard     json.RawMessage `json:"raw_card,omitempty"`
	LastSeen    time.Time       `json:"last_seen"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
