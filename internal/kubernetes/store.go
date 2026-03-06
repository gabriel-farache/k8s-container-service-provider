package kubernetes

import (
	"context"
	"fmt"
	"log/slog"

	v1alpha1 "github.com/dcm-project/k8s-container-service-provider/api/v1alpha1"
	"github.com/dcm-project/k8s-container-service-provider/internal/store"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// K8sContainerStore implements store.ContainerRepository backed by Kubernetes
// Deployments, Pods, and Services.
type K8sContainerStore struct {
	client kubernetes.Interface
	cfg    K8sConfig
	logger *slog.Logger
}

// NewK8sContainerStore creates a new K8sContainerStore with the given client, config, and logger.
func NewK8sContainerStore(client kubernetes.Interface, cfg K8sConfig, logger *slog.Logger) *K8sContainerStore {
	return &K8sContainerStore{
		client: client,
		cfg:    cfg,
		logger: logger,
	}
}

// buildContainer reconstructs an API Container from a Deployment and enriches
// it with runtime data from the cluster.
func (s *K8sContainerStore) buildContainer(ctx context.Context, deploy *appsv1.Deployment, instanceID string) (*v1alpha1.Container, error) {
	c, err := containerFromDeployment(deploy, instanceID)
	if err != nil {
		return nil, err
	}
	if err := s.enrichFromCluster(ctx, &c, deploy, instanceID); err != nil {
		return nil, err
	}
	return &c, nil
}

// enrichFromCluster enriches a Container with runtime data from Pods and Services.
func (s *K8sContainerStore) enrichFromCluster(
	ctx context.Context,
	c *v1alpha1.Container,
	deploy *appsv1.Deployment,
	instanceID string,
) error {
	pods, err := s.client.CoreV1().Pods(s.cfg.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: instanceSelector(instanceID),
	})
	if err != nil {
		return err
	}

	switch {
	case len(pods.Items) == 1:
		enrichWithPod(c, &pods.Items[0])
	case len(pods.Items) == 2 && isRollingUpdate(deploy):
		s.logger.Warn("rolling update in progress, selecting active pod",
			"instanceID", instanceID,
			"podCount", len(pods.Items),
		)
		enrichWithPod(c, selectActivePod(pods.Items))
	case len(pods.Items) > 1:
		return &store.ConflictError{
			Message: fmt.Sprintf("multiple pods found for container %q", instanceID),
		}
	default: // 0 pods
		pending := v1alpha1.PENDING
		c.Status = &pending
		if t := latestDeploymentTransitionTime(deploy); t != nil {
			c.UpdateTime = t
		}
	}

	svc, err := s.client.CoreV1().Services(s.cfg.Namespace).Get(ctx, deploy.Name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	} else {
		enrichWithService(c, svc)
	}

	return nil
}

// isRollingUpdate returns true if the Deployment is in the middle of a rollout.
func isRollingUpdate(deploy *appsv1.Deployment) bool {
	if deploy.Spec.Replicas != nil && deploy.Status.UpdatedReplicas != *deploy.Spec.Replicas {
		return true
	}
	return deploy.Status.UnavailableReplicas > 0
}

// selectActivePod returns the pod currently able to process work.
// Prefers Running pods; falls back to the most recently created pod.
func selectActivePod(pods []corev1.Pod) *corev1.Pod {
	for i := range pods {
		if pods[i].Status.Phase == corev1.PodRunning {
			return &pods[i]
		}
	}
	// No Running pod — pick the newest by creation timestamp.
	newest := &pods[0]
	for i := 1; i < len(pods); i++ {
		if pods[i].CreationTimestamp.After(newest.CreationTimestamp.Time) {
			newest = &pods[i]
		}
	}
	return newest
}
