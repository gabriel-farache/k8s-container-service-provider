// Package dcm defines shared constants and conventions for the DCM service provider.
package dcm

const (
	LabelManagedBy    = "dcm.project/managed-by"
	LabelInstanceID   = "dcm.project/dcm-instance-id"
	LabelServiceType  = "dcm.project/dcm-service-type"
	ValueManagedByDCM = "dcm"
	ValueServiceType  = "container"
)

// ReservedLabelKeys is the set of label keys managed by DCM.
// User-supplied labels must not use these keys.
var ReservedLabelKeys = map[string]bool{
	LabelManagedBy:   true,
	LabelInstanceID:  true,
	LabelServiceType: true,
}
