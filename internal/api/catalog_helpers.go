package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/model"
)

var (
	errUpsertProvider    = errors.New("failed to upsert provider")
	errDuplicateEndpoint = errors.New("an entry with this endpoint already exists")
	errCreateEntry       = errors.New("failed to create catalog entry")
)

// registerAgentType is shared post-parse logic used by both ImportCatalogEntry and
// RegisterAgentCard. It assigns server-managed fields, upserts the provider,
// wraps the result in a CatalogEntry, and persists it to the store.
//
// rawCard is used only to extract a display name from the "name" JSON field; it
// may be nil, in which case the endpoint is used as the display name.
func (h *Handler) registerAgentType(
	ctx context.Context,
	agentType *model.AgentType,
	rawCard []byte,
	source model.SourceType,
) (*model.CatalogEntry, error) {
	now := time.Now().UTC()
	agentType.ID = uuid.NewString()
	agentType.AgentKey = model.ComputeAgentKey(agentType.Protocol, agentType.Endpoint)
	agentType.CreatedOn = now

	// Upsert provider if present.
	if agentType.Provider != nil {
		if agentType.Provider.ID == "" {
			agentType.Provider.ID = uuid.NewString()
		}
		agentType.Provider.CreatedOn = now
		upserted, err := h.store.UpsertProvider(ctx, agentType.Provider)
		if err != nil {
			return nil, errUpsertProvider
		}
		agentType.Provider = upserted
		agentType.ProviderID = &upserted.ID
	}

	// Try to extract a human-readable name from the raw card JSON.
	displayName := agentType.Endpoint
	if len(rawCard) > 0 {
		var cardName struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(rawCard, &cardName) == nil && cardName.Name != "" {
			displayName = cardName.Name
		}
	}

	entry := &model.CatalogEntry{
		ID:          uuid.NewString(),
		AgentTypeID: agentType.ID,
		AgentType:   agentType,
		DisplayName: displayName,
		Source:      source,
		Status:      model.StatusUnknown,
		Validity:    model.Validity{LastSeen: now},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.store.Create(ctx, entry); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			return nil, errDuplicateEndpoint
		}
		return nil, errCreateEntry
	}

	return entry, nil
}
