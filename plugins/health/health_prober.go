package health

import (
	"context"
	"fmt"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// ProbeEntry probes an entry by ID and persists the result.
// Implements the api.HealthProber interface structurally.
func (p *Plugin) ProbeEntry(ctx context.Context, id string) (model.Health, error) {
	entry, err := p.store.Get(ctx, id)
	if err != nil {
		return model.Health{}, fmt.Errorf("getting entry for probe: %w", err)
	}
	if entry == nil {
		return model.Health{}, model.ErrEntryNotFound
	}
	h := p.probeOne(ctx, entry)
	if err := p.store.UpdateHealth(ctx, id, h); err != nil {
		p.log.Warn("failed to persist on-demand probe", "id", id, "err", err)
	}
	return h, nil
}

// ProbeOneForTest exposes probeOne for white-box unit tests.
func (p *Plugin) ProbeOneForTest(ctx context.Context, entry *model.CatalogEntry) (model.Health, error) {
	return p.probeOne(ctx, entry), nil
}
