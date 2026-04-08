package cardstore_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/plugins/cardstore"
)

func newTestPlugin(t *testing.T) *cardstore.Plugin {
	t.Helper()
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("opening in-memory db: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := database.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	p := cardstore.New(database)
	if err := p.MigrateSchema(context.Background()); err != nil {
		t.Fatalf("migrating schema: %v", err)
	}
	return p
}

func TestCardPlugin_Name(t *testing.T) {
	p := newTestPlugin(t)
	if got := p.Name(); got != "card-store" {
		t.Errorf("Name() = %q; want %q", got, "card-store")
	}
}

func TestCardPlugin_Type(t *testing.T) {
	p := newTestPlugin(t)
	if got := p.Type(); got != kernel.PluginTypeCardStore {
		t.Errorf("Type() = %q; want %q", got, kernel.PluginTypeCardStore)
	}
}

func TestCardStore_StoreAndGet(t *testing.T) {
	p := newTestPlugin(t)
	ctx := context.Background()

	data := []byte(`{"name":"test-agent"}`)
	contentType := "application/json"

	if err := p.StoreCard(ctx, "agent-001", data, contentType); err != nil {
		t.Fatalf("StoreCard() error = %v", err)
	}

	card, err := p.GetCard(ctx, "agent-001")
	if err != nil {
		t.Fatalf("GetCard() error = %v", err)
	}

	if card.AgentTypeID != "agent-001" {
		t.Errorf("AgentTypeID = %q; want %q", card.AgentTypeID, "agent-001")
	}
	if !bytes.Equal(card.Data, data) {
		t.Errorf("Data mismatch: got %q, want %q", card.Data, data)
	}
	if card.ContentType != contentType {
		t.Errorf("ContentType = %q; want %q", card.ContentType, contentType)
	}
	if card.Truncated {
		t.Error("Truncated should be false for small payload")
	}
}

func TestCardStore_GetNotFound(t *testing.T) {
	p := newTestPlugin(t)
	ctx := context.Background()

	_, err := p.GetCard(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("GetCard() expected error for nonexistent ID, got nil")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetCard() error should wrap gorm.ErrRecordNotFound; got: %v", err)
	}
}

func TestCardStore_Truncation(t *testing.T) {
	p := newTestPlugin(t)
	ctx := context.Background()

	// 257 KiB payload — should be truncated to 256 KiB
	payload := bytes.Repeat([]byte("x"), 257*1024)

	if err := p.StoreCard(ctx, "agent-trunc", payload, "application/json"); err != nil {
		t.Fatalf("StoreCard() error = %v", err)
	}

	card, err := p.GetCard(ctx, "agent-trunc")
	if err != nil {
		t.Fatalf("GetCard() error = %v", err)
	}

	const want = 256 * 1024
	if len(card.Data) != want {
		t.Errorf("len(Data) = %d; want %d", len(card.Data), want)
	}
	if !card.Truncated {
		t.Error("Truncated should be true for oversized payload")
	}
}

func TestCardStore_UpsertUpdates(t *testing.T) {
	p := newTestPlugin(t)
	ctx := context.Background()

	first := []byte(`{"version":"1"}`)
	second := []byte(`{"version":"2"}`)

	if err := p.StoreCard(ctx, "agent-upsert", first, "application/json"); err != nil {
		t.Fatalf("StoreCard() first: %v", err)
	}
	if err := p.StoreCard(ctx, "agent-upsert", second, "application/json"); err != nil {
		t.Fatalf("StoreCard() second: %v", err)
	}

	card, err := p.GetCard(ctx, "agent-upsert")
	if err != nil {
		t.Fatalf("GetCard() error = %v", err)
	}

	if !bytes.Equal(card.Data, second) {
		t.Errorf("Data = %q; want %q (second store should overwrite)", card.Data, second)
	}
}
