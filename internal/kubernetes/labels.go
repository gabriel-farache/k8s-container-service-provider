package kubernetes

import (
	"github.com/dcm-project/k8s-container-service-provider/internal/dcm"
	"k8s.io/apimachinery/pkg/labels"
)

// dcmLabels returns the standard DCM labels for a given instance ID.
func dcmLabels(instanceID string) map[string]string {
	return map[string]string{
		dcm.LabelManagedBy:   dcm.ValueManagedByDCM,
		dcm.LabelInstanceID:  instanceID,
		dcm.LabelServiceType: dcm.ValueServiceType,
	}
}

// mergeLabels merges DCM base labels with user labels into a new map.
// Base labels overwrite user labels on collision — DCM labels always win
// (defense-in-depth against label corruption).
func mergeLabels(base, user map[string]string) map[string]string {
	return labels.Merge(labels.Set(user), labels.Set(base))
}

// instanceSelector returns a label selector string that matches a specific
// DCM instance by ID.
func instanceSelector(instanceID string) string {
	return labels.Set{
		dcm.LabelInstanceID: instanceID,
		dcm.LabelManagedBy:  dcm.ValueManagedByDCM,
	}.String()
}

// dcmSelector returns a label selector string that matches all DCM-managed
// container resources.
func dcmSelector() string {
	return labels.Set{
		dcm.LabelManagedBy:   dcm.ValueManagedByDCM,
		dcm.LabelServiceType: dcm.ValueServiceType,
	}.String()
}
