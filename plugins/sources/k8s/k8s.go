// Package k8s provides the Kubernetes service discovery source plugin.
package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/PawelHaracz/agentlens/internal/discovery"
	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
)

const (
	annotationAgentType = "agentlens.io/type"
	annotationCardPath  = "agentlens.io/card-path"
	annotationTags      = "agentlens.io/tags"
	annotationTeam      = "agentlens.io/team"
)

// Plugin implements the Kubernetes source plugin.
type Plugin struct {
	client     kubernetes.Interface
	namespaces []string
	crawler    *discovery.Crawler
	kern       kernel.Kernel
	log        *slog.Logger
}

// New creates a new K8s source plugin.
func New(client kubernetes.Interface, namespaces []string) *Plugin {
	return &Plugin{
		client:     client,
		namespaces: namespaces,
	}
}

// Name returns the plugin name.
func (p *Plugin) Name() string { return "k8s-source" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeSource }

// Init initializes the plugin.
func (p *Plugin) Init(k kernel.Kernel) error {
	p.kern = k
	p.crawler = discovery.NewCrawler()
	p.log = k.Logger().With("component", "k8s-source")
	return nil
}

// Start starts the plugin (no-op, discovery manager drives it).
func (p *Plugin) Start(ctx context.Context) error { return nil }

// Stop stops the plugin (no-op).
func (p *Plugin) Stop(ctx context.Context) error { return nil }

// Discover lists Kubernetes Services and fetches agent cards for annotated ones.
func (p *Plugin) Discover(ctx context.Context) ([]*model.CatalogEntry, error) {
	var entries []*model.CatalogEntry
	for _, ns := range p.namespaces {
		svcs, err := p.client.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing services in namespace %s: %w", ns, err)
		}
		for _, svc := range svcs.Items {
			entry, err := p.processService(ctx, svc)
			if err != nil {
				p.log.Warn("failed to process service", "name", svc.Name, "namespace", svc.Namespace, "err", err)
				continue
			}
			if entry != nil {
				entries = append(entries, entry)
			}
		}
	}
	return entries, nil
}

func (p *Plugin) processService(ctx context.Context, svc corev1.Service) (*model.CatalogEntry, error) {
	ann := svc.Annotations
	if ann == nil {
		return nil, nil
	}
	agentType, ok := ann[annotationAgentType]
	if !ok {
		return nil, nil
	}

	protocol := model.Protocol(agentType)
	parser, ok := p.kern.Parser(protocol)
	if !ok {
		return nil, fmt.Errorf("no parser for protocol %s", agentType)
	}

	cardPath := ann[annotationCardPath]
	if cardPath == "" {
		cardPath = parser.CardPath()
	}

	port := p.firstPort(svc)
	url := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d%s",
		svc.Name, svc.Namespace, port, cardPath)

	raw, err := p.crawler.FetchCard(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("fetching card from %s: %w", url, err)
	}

	entry, err := parser.Parse(raw, model.SourceK8s)
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

func (p *Plugin) firstPort(svc corev1.Service) int32 {
	for _, port := range svc.Spec.Ports {
		return port.Port
	}
	return 80
}
