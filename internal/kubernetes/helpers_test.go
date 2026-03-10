package kubernetes_test

import (
	"context"
	"io"
	"log/slog"
	"time"

	v1alpha1 "github.com/dcm-project/k8s-container-service-provider/api/v1alpha1"
	"github.com/dcm-project/k8s-container-service-provider/internal/dcm"
	k8sstore "github.com/dcm-project/k8s-container-service-provider/internal/kubernetes"
	"github.com/dcm-project/k8s-container-service-provider/internal/store"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// Compile-time assertion: K8sContainerStore implements ContainerRepository (TC-U024).
var _ store.ContainerRepository = (*k8sstore.K8sContainerStore)(nil)

// newTestStore creates a K8sContainerStore backed by a fake clientset.
func newTestStore(cfg k8sstore.K8sConfig) (*k8sstore.K8sContainerStore, *fake.Clientset) {
	client := fake.NewClientset()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	s := k8sstore.NewK8sContainerStore(client, cfg, logger)
	return s, client
}

// defaultConfig returns a K8sConfig with reasonable defaults for testing.
func defaultConfig() k8sstore.K8sConfig {
	return k8sstore.K8sConfig{
		Namespace:          "default",
		DefaultServiceType: "ClusterIP",
	}
}

// minimalContainer creates a container with only the required fields set.
func minimalContainer(name string) v1alpha1.Container {
	return v1alpha1.Container{
		ServiceType: v1alpha1.ContainerServiceTypeContainer,
		Metadata: v1alpha1.ContainerMetadata{
			Name: name,
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

// containerWithPorts creates a container with the specified network ports (visibility=none).
func containerWithPorts(name string, ports ...int) v1alpha1.Container {
	c := minimalContainer(name)
	containerPorts := make([]v1alpha1.ContainerPort, len(ports))
	for i, p := range ports {
		containerPorts[i] = v1alpha1.ContainerPort{
			ContainerPort: p,
			Visibility:    v1alpha1.None,
		}
	}
	c.Network = &v1alpha1.ContainerNetwork{
		Ports: containerPorts,
	}
	return c
}

// containerWithVisiblePorts creates a container where all ports share the given visibility.
func containerWithVisiblePorts(name string, visibility v1alpha1.ContainerPortVisibility, ports ...int) v1alpha1.Container {
	c := minimalContainer(name)
	containerPorts := make([]v1alpha1.ContainerPort, len(ports))
	for i, p := range ports {
		containerPorts[i] = v1alpha1.ContainerPort{
			ContainerPort: p,
			Visibility:    visibility,
		}
	}
	c.Network = &v1alpha1.ContainerNetwork{
		Ports: containerPorts,
	}
	return c
}

// dcmLabels returns the standard DCM labels for a given instance ID.
func dcmLabels(instanceID string) map[string]string {
	return map[string]string{
		dcm.LabelManagedBy:   dcm.ValueManagedByDCM,
		dcm.LabelInstanceID:  instanceID,
		dcm.LabelServiceType: dcm.ValueServiceType,
	}
}

// --- Deployment helpers with functional options ---

type fakeDeployOption func(*appsv1.Deployment)

func withDeploymentConditions(conditions []appsv1.DeploymentCondition) fakeDeployOption {
	return func(d *appsv1.Deployment) { d.Status.Conditions = conditions }
}

func withDeploymentStatus(status appsv1.DeploymentStatus) fakeDeployOption {
	return func(d *appsv1.Deployment) { d.Status = status }
}

func createFakeDeployment(client kubernetes.Interface, namespace, name, instanceID string, opts ...fakeDeployOption) error {
	labels := dcmLabels(instanceID)
	replicas := int32(1)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  name,
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
	for _, opt := range opts {
		opt(deploy)
	}
	_, err := client.AppsV1().Deployments(namespace).Create(context.Background(), deploy, metav1.CreateOptions{})
	return err
}

// --- Pod helpers with functional options ---

type fakePodOption func(*corev1.Pod)

func withPodConditions(conditions []corev1.PodCondition) fakePodOption {
	return func(p *corev1.Pod) { p.Status.Conditions = conditions }
}

func withCreationTime(t time.Time) fakePodOption {
	return func(p *corev1.Pod) { p.CreationTimestamp = metav1.NewTime(t) }
}

func createFakePod(client kubernetes.Interface, namespace, name, instanceID string, phase corev1.PodPhase, podIP string, opts ...fakePodOption) error {
	labels := dcmLabels(instanceID)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Status: corev1.PodStatus{
			Phase: phase,
			PodIP: podIP,
		},
	}
	for _, opt := range opts {
		opt(pod)
	}
	_, err := client.CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
	return err
}

// --- Service helpers with functional options ---

type fakeServiceOption func(*corev1.Service)

func withClusterIP(ip string) fakeServiceOption {
	return func(svc *corev1.Service) { svc.Spec.ClusterIP = ip }
}

func withLoadBalancerIP(ip string) fakeServiceOption {
	return func(svc *corev1.Service) {
		svc.Spec.Type = corev1.ServiceTypeLoadBalancer
		svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: ip}}
	}
}

func createFakeService(client kubernetes.Interface, namespace, name, instanceID string, svcType corev1.ServiceType, ports []int32, opts ...fakeServiceOption) error {
	labels := dcmLabels(instanceID)
	svcPorts := make([]corev1.ServicePort, len(ports))
	for i, p := range ports {
		svcPorts[i] = corev1.ServicePort{
			Port:       p,
			TargetPort: intstr.FromInt32(p),
			Protocol:   corev1.ProtocolTCP,
		}
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: labels,
			Ports:    svcPorts,
		},
	}
	for _, opt := range opts {
		opt(svc)
	}
	_, err := client.CoreV1().Services(namespace).Create(context.Background(), svc, metav1.CreateOptions{})
	return err
}
