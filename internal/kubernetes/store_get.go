package kubernetes

import (
	"context"
	"fmt"

	v1alpha1 "github.com/dcm-project/k8s-container-service-provider/api/v1alpha1"
	"github.com/dcm-project/k8s-container-service-provider/internal/store"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Get retrieves a container by its instance ID, enriching it with runtime
// data from Pods and Services.
func (s *K8sContainerStore) Get(ctx context.Context, containerID string) (*v1alpha1.Container, error) {
	deploys, err := s.client.AppsV1().Deployments(s.cfg.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: instanceSelector(containerID),
	})
	if err != nil {
		return nil, err
	}
	if len(deploys.Items) == 0 {
		return nil, &store.NotFoundError{ID: containerID}
	}
	if len(deploys.Items) > 1 {
		return nil, &store.ConflictError{Message: fmt.Sprintf("multiple deployments found for container %q", containerID)}
	}

	deploy := &deploys.Items[0]
	return s.buildContainer(ctx, deploy, containerID)
}
