package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/PawelHaracz/agentlens/internal/model"
)

const (
	annotationAgentType = "agentlens.io/type"
	annotationCardPath  = "agentlens.io/card-path"
	annotationTags      = "agentlens.io/tags"
	annotationTeam      = "agentlens.io/team"

	defaultA2ACardPath = "/.well-known/agent-card.json"
	defaultMCPCardPath = "/.well-known/mcp/server.json"
)

// K8sSource discovers agents by inspecting Kubernetes Service annotations.
type K8sSource struct {
	client     kubernetes.Interface
	namespaces []string
	crawler    *Crawler
	log        *slog.Logger
}

// NewK8sSource creates a K8sSource for the given namespaces.
func NewK8sSource(client kubernetes.Interface, namespaces []string) *K8sSource {
	return &K8sSource{
		client:     client,
		namespaces: namespaces,
		crawler:    NewCrawler(),
		log:        slog.With("component", "k8s-source"),
	}
}

// Name returns the source identifier.
func (k *K8sSource) Name() string { return "k8s" }

// Discover lists Kubernetes Services and fetches agent cards for annotated ones.
func (k *K8sSource) Discover(ctx context.Context) ([]*model.CatalogEntry, error) {
	var entries []*model.CatalogEntry
	for _, ns := range k.namespaces {
		svcs, err := k.client.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing services in namespace %s: %w", ns, err)
		}
		for _, svc := range svcs.Items {
			entry, err := k.processService(ctx, svc)
			if err != nil {
				k.log.Warn("failed to process service", "name", svc.Name, "namespace", svc.Namespace, "err", err)
				continue
			}
			if entry != nil {
				entries = append(entries, entry)
			}
		}
	}
	return entries, nil
}

func (k *K8sSource) processService(ctx context.Context, svc corev1.Service) (*model.CatalogEntry, error) {
	ann := svc.Annotations
	if ann == nil {
		return nil, nil
	}
	agentType, ok := ann[annotationAgentType]
	if !ok {
		return nil, nil
	}

	cardPath := ann[annotationCardPath]
	if cardPath == "" {
		switch agentType {
		case "mcp":
			cardPath = defaultMCPCardPath
		default:
			cardPath = defaultA2ACardPath
		}
	}

	port := k.firstPort(svc)
	url := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d%s",
		svc.Name, svc.Namespace, port, cardPath)

	raw, err := k.crawler.FetchCard(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("fetching card from %s: %w", url, err)
	}

	var entry *model.CatalogEntry
	switch agentType {
	case "mcp":
		entry, err = ParseMCPCard(raw, model.SourceK8s)
	default:
		entry, err = ParseA2ACard(raw, model.SourceK8s)
	}
	if err != nil {
		return nil, fmt.Errorf("parsing card: %w", err)
	}

	if entry.Metadata == nil {
		entry.Metadata = make(map[string]string)
	}
	entry.Metadata["kubernetes.namespace"] = svc.Namespace
	if team := ann[annotationTeam]; team != "" {
		entry.Provider.Team = team
	}
	if tags := ann[annotationTags]; tags != "" {
		entry.Categories = strings.Split(tags, ",")
	}

	return entry, nil
}

func (k *K8sSource) firstPort(svc corev1.Service) int32 {
	for _, p := range svc.Spec.Ports {
		return p.Port
	}
	return 80
}
