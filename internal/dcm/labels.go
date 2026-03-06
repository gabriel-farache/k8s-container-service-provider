package dcm

const (
	LabelManagedBy    = "managed-by"
	LabelInstanceID   = "dcm-instance-id"
	LabelServiceType  = "dcm-service-type"
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
