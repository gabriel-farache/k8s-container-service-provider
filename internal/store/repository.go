package store

import (
	"context"

	v1alpha1 "github.com/dcm-project/k8s-container-service-provider/api/v1alpha1"
)

// ContainerRepository defines the storage interface for container CRUD operations.
type ContainerRepository interface {
	Create(ctx context.Context, container v1alpha1.Container, id string) (*v1alpha1.Container, error)
	Get(ctx context.Context, containerID string) (*v1alpha1.Container, error)
	List(ctx context.Context, maxPageSize int32, pageToken string) (*v1alpha1.ContainerList, error)
	Delete(ctx context.Context, containerID string) error
}
