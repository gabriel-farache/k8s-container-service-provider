package kubernetes

import (
	"fmt"
	"time"

	v1alpha1 "github.com/dcm-project/k8s-container-service-provider/api/v1alpha1"
	"github.com/dcm-project/k8s-container-service-provider/internal/dcm"
	"github.com/dcm-project/k8s-container-service-provider/internal/units"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// MapPodPhaseToStatus maps a Kubernetes Pod phase to a container status.
// Returns the mapped status and true if mapping exists, or ("", false)
// for phases that should be ignored (e.g., Succeeded per DD-020).
func MapPodPhaseToStatus(phase corev1.PodPhase) (v1alpha1.ContainerStatus, bool) {
	switch phase {
	case corev1.PodPending:
		return v1alpha1.PENDING, true
	case corev1.PodRunning:
		return v1alpha1.RUNNING, true
	case corev1.PodFailed:
		return v1alpha1.FAILED, true
	case corev1.PodUnknown:
		return v1alpha1.UNKNOWN, true
	default:
		// Succeeded and any unknown phases are not mapped
		return "", false
	}
}

// containerFromDeployment reconstructs an API Container from a Kubernetes Deployment.
// It reverse-maps Deployment spec fields back to the API representation.
func containerFromDeployment(deploy *appsv1.Deployment, instanceID string) (v1alpha1.Container, error) {
	containers := deploy.Spec.Template.Spec.Containers
	id := instanceID
	path := fmt.Sprintf("containers/%s", instanceID)
	ns := deploy.Namespace
	createTime := deploy.CreationTimestamp.Time
	serviceType := v1alpha1.ContainerServiceTypeContainer

	k8sC := containers[0]

	c := v1alpha1.Container{
		Id:          &id,
		Path:        &path,
		ServiceType: serviceType,
		CreateTime:  &createTime,
		Image:       v1alpha1.ContainerImage{Reference: k8sC.Image},
		Metadata: v1alpha1.ContainerMetadata{
			Name:      deploy.Name,
			Namespace: &ns,
		},
	}

	// Reconstruct resources from K8s resource requirements.
	c.Resources = resourcesFromContainer(k8sC)

	// Reconstruct process (command/args/env) if present.
	if proc := processFromContainer(k8sC); proc != nil {
		c.Process = proc
	}

	// Reconstruct network ports if present.
	// Visibility is set to "none" by default; enrichWithService will update it later.
	if len(k8sC.Ports) > 0 {
		ports := make([]v1alpha1.ContainerPort, len(k8sC.Ports))
		for i, p := range k8sC.Ports {
			ports[i] = v1alpha1.ContainerPort{
				ContainerPort: int(p.ContainerPort),
				Visibility:    v1alpha1.None,
			}
		}
		c.Network = &v1alpha1.ContainerNetwork{Ports: &ports}
	}

	// Reconstruct user labels by filtering out DCM reserved labels.
	if userLabels := userLabelsFromDeployment(deploy); len(userLabels) > 0 {
		c.Metadata.Labels = &userLabels
	}

	return c, nil
}

// resourcesFromContainer extracts CPU and memory resources from a K8s container spec.
func resourcesFromContainer(k8sC corev1.Container) v1alpha1.ContainerResources {
	res := v1alpha1.ContainerResources{}

	if req, ok := k8sC.Resources.Requests[corev1.ResourceCPU]; ok {
		res.Cpu.Min = int(req.Value())
	}
	if lim, ok := k8sC.Resources.Limits[corev1.ResourceCPU]; ok {
		res.Cpu.Max = int(lim.Value())
	}
	if req, ok := k8sC.Resources.Requests[corev1.ResourceMemory]; ok {
		res.Memory.Min = units.MemoryQuantityToAPI(req)
	}
	if lim, ok := k8sC.Resources.Limits[corev1.ResourceMemory]; ok {
		res.Memory.Max = units.MemoryQuantityToAPI(lim)
	}

	return res
}

// processFromContainer extracts process configuration from a K8s container spec.
// Returns nil if no command, args, or env are set.
func processFromContainer(k8sC corev1.Container) *v1alpha1.ContainerProcess {
	if len(k8sC.Command) == 0 && len(k8sC.Args) == 0 && len(k8sC.Env) == 0 {
		return nil
	}

	proc := &v1alpha1.ContainerProcess{}
	if len(k8sC.Command) > 0 {
		cmd := make([]string, len(k8sC.Command))
		copy(cmd, k8sC.Command)
		proc.Command = &cmd
	}
	if len(k8sC.Args) > 0 {
		args := make([]string, len(k8sC.Args))
		copy(args, k8sC.Args)
		proc.Args = &args
	}
	if len(k8sC.Env) > 0 {
		envVars := make([]v1alpha1.ContainerEnvVar, len(k8sC.Env))
		for i, e := range k8sC.Env {
			envVars[i] = v1alpha1.ContainerEnvVar{Name: e.Name, Value: e.Value}
		}
		proc.Env = &envVars
	}
	return proc
}

// userLabelsFromDeployment extracts user-defined labels by filtering out DCM reserved labels.
func userLabelsFromDeployment(deploy *appsv1.Deployment) map[string]string {
	labels := make(map[string]string)
	for k, v := range deploy.Labels {
		if !dcm.ReservedLabelKeys[k] {
			labels[k] = v
		}
	}
	return labels
}

// enrichWithPod populates runtime data from a Pod into the Container.
func enrichWithPod(container *v1alpha1.Container, pod *corev1.Pod) {
	if status, ok := MapPodPhaseToStatus(pod.Status.Phase); ok {
		container.Status = &status
	}

	if pod.Status.PodIP != "" {
		if container.Network == nil {
			container.Network = &v1alpha1.ContainerNetwork{}
		}
		container.Network.Ip = &pod.Status.PodIP
	}

	if t := latestPodTransitionTime(pod); t != nil {
		container.UpdateTime = t
	}
}

// enrichWithService populates service info from a Kubernetes Service and
// infers port visibility from the Service state.
func enrichWithService(container *v1alpha1.Container, svc *corev1.Service) {
	info := &v1alpha1.ServiceInfo{}

	if svc.Spec.ClusterIP != "" {
		info.ClusterIp = &svc.Spec.ClusterIP
	}

	svcType := v1alpha1.ServiceInfoType(svc.Spec.Type)
	info.Type = &svcType

	// Build a set of target ports exposed by the Service.
	svcTargetPorts := make(map[int]bool)
	if len(svc.Spec.Ports) > 0 {
		ports := make([]v1alpha1.ServicePort, len(svc.Spec.Ports))
		for i, p := range svc.Spec.Ports {
			protocol := string(p.Protocol)
			ports[i] = v1alpha1.ServicePort{
				Port:       int(p.Port),
				TargetPort: p.TargetPort.IntValue(),
				Protocol:   &protocol,
			}
			svcTargetPorts[p.TargetPort.IntValue()] = true
		}
		info.Ports = &ports
	}

	if len(svc.Status.LoadBalancer.Ingress) > 0 && svc.Status.LoadBalancer.Ingress[0].IP != "" {
		info.ExternalIp = &svc.Status.LoadBalancer.Ingress[0].IP
	}

	container.Service = info

	// Infer port visibility from Service state.
	if container.Network != nil && container.Network.Ports != nil {
		visibility := inferVisibility(svc.Spec.Type)
		ports := *container.Network.Ports
		for i := range ports {
			if svcTargetPorts[ports[i].ContainerPort] {
				ports[i].Visibility = visibility
			}
			// Ports not in the Service keep their default (none).
		}
		container.Network.Ports = &ports
	}
}

// inferVisibility maps a K8s Service type to a port visibility value.
func inferVisibility(svcType corev1.ServiceType) v1alpha1.ContainerPortVisibility {
	switch svcType {
	case corev1.ServiceTypeLoadBalancer, corev1.ServiceTypeNodePort:
		return v1alpha1.External
	default:
		return v1alpha1.Internal
	}
}

// latestPodTransitionTime returns the most recent LastTransitionTime from Pod conditions.
func latestPodTransitionTime(pod *corev1.Pod) *time.Time {
	var latest *time.Time
	for i := range pod.Status.Conditions {
		t := pod.Status.Conditions[i].LastTransitionTime.Time
		if t.IsZero() {
			continue
		}
		if latest == nil || t.After(*latest) {
			latest = &t
		}
	}
	return latest
}

// latestDeploymentTransitionTime returns the most recent LastTransitionTime from Deployment conditions.
func latestDeploymentTransitionTime(deploy *appsv1.Deployment) *time.Time {
	var latest *time.Time
	for i := range deploy.Status.Conditions {
		t := deploy.Status.Conditions[i].LastTransitionTime.Time
		if t.IsZero() {
			continue
		}
		if latest == nil || t.After(*latest) {
			latest = &t
		}
	}
	return latest
}

// buildDeployment creates a Kubernetes Deployment from a Container spec.
func buildDeployment(container v1alpha1.Container, id string, cfg K8sConfig, labels map[string]string) *appsv1.Deployment {
	replicas := int32(1)

	// Selector uses only DCM labels (immutable after creation)
	selectorLabels := dcmLabels(id)

	// CPU resources
	cpuReq, cpuLim := units.ConvertCPU(container.Resources.Cpu)

	// Memory resources — errors handled upstream; safe to ignore here since
	// validation occurs before buildDeployment is called.
	memReq, _ := units.ConvertMemory(container.Resources.Memory.Min)
	memLim, _ := units.ConvertMemory(container.Resources.Memory.Max)

	k8sContainer := corev1.Container{
		Name:  container.Metadata.Name,
		Image: container.Image.Reference,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    cpuReq,
				corev1.ResourceMemory: memReq,
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    cpuLim,
				corev1.ResourceMemory: memLim,
			},
		},
	}

	if container.Process != nil {
		if container.Process.Command != nil {
			k8sContainer.Command = *container.Process.Command
		}
		if container.Process.Args != nil {
			k8sContainer.Args = *container.Process.Args
		}
		if container.Process.Env != nil {
			envVars := make([]corev1.EnvVar, len(*container.Process.Env))
			for i, e := range *container.Process.Env {
				envVars[i] = corev1.EnvVar{Name: e.Name, Value: e.Value}
			}
			k8sContainer.Env = envVars
		}
	}

	if container.Network != nil && container.Network.Ports != nil && len(*container.Network.Ports) > 0 {
		ports := make([]corev1.ContainerPort, len(*container.Network.Ports))
		for i, p := range *container.Network.Ports {
			ports[i] = corev1.ContainerPort{
				ContainerPort: int32(p.ContainerPort),
			}
		}
		k8sContainer.Ports = ports
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      container.Metadata.Name,
			Namespace: cfg.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{k8sContainer},
				},
			},
		},
	}
}

// buildService creates a Kubernetes Service from a Container spec.
// servicePorts contains only the ports with non-none visibility.
func buildService(container v1alpha1.Container, id string, cfg K8sConfig, labels map[string]string, svcType corev1.ServiceType, servicePorts []v1alpha1.ContainerPort) *corev1.Service {
	// Selector uses only DCM labels
	selectorLabels := dcmLabels(id)

	svcPorts := make([]corev1.ServicePort, len(servicePorts))
	for i, p := range servicePorts {
		svcPorts[i] = corev1.ServicePort{
			Name:       fmt.Sprintf("port-%d", p.ContainerPort),
			Port:       int32(p.ContainerPort),
			TargetPort: intstr.FromInt32(int32(p.ContainerPort)),
			Protocol:   corev1.ProtocolTCP,
		}
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      container.Metadata.Name,
			Namespace: cfg.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: selectorLabels,
			Ports:    svcPorts,
		},
	}
}
