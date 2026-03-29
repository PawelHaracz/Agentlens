package discovery_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/PawelHaracz/agentlens/internal/discovery"
	"github.com/PawelHaracz/agentlens/internal/model"
)

func TestK8sSource_NoAnnotations(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "plain-svc", Namespace: "default"},
	})
	src := discovery.NewK8sSource(client, []string{"default"})
	agents, err := src.Discover(context.Background())
	require.NoError(t, err)
	assert.Empty(t, agents)
}

func TestK8sSource_WithAnnotation(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-svc",
			Namespace: "default",
			Annotations: map[string]string{
				"agentlens.io/type": "a2a",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{Port: 8080}},
		},
	})
	src := discovery.NewK8sSource(client, []string{"default"})
	// This will fail to fetch (not actually running in k8s) but shouldn't error out
	agents, err := src.Discover(context.Background())
	require.NoError(t, err)
	// The fetch will fail since we're not in k8s, but agent list should be empty (not error)
	assert.Empty(t, agents)
	_ = model.ProtocolA2A // ensure import is used
}
