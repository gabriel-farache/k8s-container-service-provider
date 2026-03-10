package container_test

import (
	"context"
	"time"

	v1alpha1 "github.com/dcm-project/k8s-container-service-provider/api/v1alpha1"
	oapigen "github.com/dcm-project/k8s-container-service-provider/internal/api/server"
	"github.com/dcm-project/k8s-container-service-provider/internal/handlers/container"
	"github.com/dcm-project/k8s-container-service-provider/internal/store"
)

// ---------------------------------------------------------------------------
// Compile-time assertions
// ---------------------------------------------------------------------------

// TC-U009 (via TC-U008): Handler implements StrictServerInterface.
var _ oapigen.StrictServerInterface = (*container.Handler)(nil)

// mockContainerRepository implements store.ContainerRepository.
var _ store.ContainerRepository = (*mockContainerRepository)(nil)

// ---------------------------------------------------------------------------
// Mock repository
// ---------------------------------------------------------------------------

// mockContainerRepository is a test double for store.ContainerRepository.
// Each method delegates to a configurable function field. Unconfigured methods
// panic so that unexpected calls are immediately visible.
type mockContainerRepository struct {
	CreateFunc func(ctx context.Context, container v1alpha1.Container, id string) (*v1alpha1.Container, error)
	GetFunc    func(ctx context.Context, containerID string) (*v1alpha1.Container, error)
	ListFunc   func(ctx context.Context, maxPageSize int32, pageToken string) (*v1alpha1.ContainerList, error)
	DeleteFunc func(ctx context.Context, containerID string) error
}

func (m *mockContainerRepository) Create(ctx context.Context, container v1alpha1.Container, id string) (*v1alpha1.Container, error) {
	if m.CreateFunc == nil {
		panic("unexpected call to Create")
	}
	return m.CreateFunc(ctx, container, id)
}

func (m *mockContainerRepository) Get(ctx context.Context, containerID string) (*v1alpha1.Container, error) {
	if m.GetFunc == nil {
		panic("unexpected call to Get")
	}
	return m.GetFunc(ctx, containerID)
}

func (m *mockContainerRepository) List(ctx context.Context, maxPageSize int32, pageToken string) (*v1alpha1.ContainerList, error) {
	if m.ListFunc == nil {
		panic("unexpected call to List")
	}
	return m.ListFunc(ctx, maxPageSize, pageToken)
}

func (m *mockContainerRepository) Delete(ctx context.Context, containerID string) error {
	if m.DeleteFunc == nil {
		panic("unexpected call to Delete")
	}
	return m.DeleteFunc(ctx, containerID)
}

// ---------------------------------------------------------------------------
// Test data helpers
// ---------------------------------------------------------------------------

// validCreateBody returns a Container with all required fields populated,
// suitable for use as a CreateContainer request body.
func validCreateBody() v1alpha1.Container {
	return v1alpha1.Container{
		ServiceType: v1alpha1.ContainerServiceTypeContainer,
		Metadata: v1alpha1.ContainerMetadata{
			Name: "my-container",
		},
		Image: v1alpha1.ContainerImage{
			Reference: "nginx:latest",
		},
		Resources: v1alpha1.ContainerResources{
			Cpu: v1alpha1.ContainerCpu{
				Min: 1,
				Max: 2,
			},
			Memory: v1alpha1.ContainerMemory{
				Min: "1GB",
				Max: "2GB",
			},
		},
	}
}

// newContainerResult simulates the enriched output the store returns after a
// successful Create. Read-only fields (id, path, status, timestamps, namespace)
// are populated as the real store would set them.
func newContainerResult(c v1alpha1.Container, id, namespace string) *v1alpha1.Container {
	now := time.Now().UTC()
	status := v1alpha1.PENDING
	path := "containers/" + id

	result := c
	result.Id = &id
	result.Path = &path
	result.Status = &status
	result.CreateTime = &now
	result.UpdateTime = &now
	result.Metadata.Namespace = &namespace
	return &result
}
