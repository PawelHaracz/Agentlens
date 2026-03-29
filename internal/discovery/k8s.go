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
func (k *K8sSource) Discover(ctx context.Context) ([]*model.Agent, error) {
	var agents []*model.Agent
	for _, ns := range k.namespaces {
		svcs, err := k.client.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing services in namespace %s: %w", ns, err)
		}
		for _, svc := range svcs.Items {
			agent, err := k.processService(ctx, svc)
			if err != nil {
				k.log.Warn("failed to process service", "name", svc.Name, "namespace", svc.Namespace, "err", err)
				continue
			}
			if agent != nil {
				agents = append(agents, agent)
			}
		}
	}
	return agents, nil
}

func (k *K8sSource) processService(ctx context.Context, svc corev1.Service) (*model.Agent, error) {
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

	var agent *model.Agent
	switch agentType {
	case "mcp":
		agent, err = ParseMCPCard(raw, model.SourceK8s)
	default:
		agent, err = ParseA2ACard(raw, model.SourceK8s)
	}
	if err != nil {
		return nil, fmt.Errorf("parsing card: %w", err)
	}

	agent.Namespace = svc.Namespace
	if team := ann[annotationTeam]; team != "" {
		agent.Team = team
	}
	if tags := ann[annotationTags]; tags != "" {
		agent.Tags = strings.Split(tags, ",")
	}

	return agent, nil
}

func (k *K8sSource) firstPort(svc corev1.Service) int32 {
	for _, p := range svc.Spec.Ports {
		return p.Port
	}
	return 80
}
