package cloud

import "time"

// NATSDeploymentState is the lifecycle of a registered deployment in Cloud.
type NATSDeploymentState string

const (
	NATSDeploymentUnregistered NATSDeploymentState = "unregistered"
	NATSDeploymentReady        NATSDeploymentState = "ready"
	NATSDeploymentUnreachable  NATSDeploymentState = "unreachable"
	NATSDeploymentIncompatible NATSDeploymentState = "incompatible"
	NATSDeploymentDegraded     NATSDeploymentState = "degraded"
	NATSDeploymentDraining     NATSDeploymentState = "draining"
)

// CredentialReference points to a secret-managed credential without exposing
// the secret value in Cloud state.
type CredentialReference struct {
	ID          string    `json:"id"`
	Provider    string    `json:"provider"`
	SecretRef   string    `json:"secret_ref"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// NATSDeploymentSpec declares the operator-hosted NATS target Cloud should
// register and validate.
type NATSDeploymentSpec struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	Region             string              `json:"region"`
	Servers            []string            `json:"servers"`
	AccountMode        string              `json:"account_mode"`
	TLSRequired        bool                `json:"tls_required"`
	JetStreamRequired  bool                `json:"jetstream_required"`
	BackupPolicyRef    string              `json:"backup_policy_ref"`
	RPOSeconds         int                 `json:"rpo_seconds"`
	RTOSeconds         int                 `json:"rto_seconds"`
	CredentialRef      CredentialReference `json:"credential_ref"`
	StreamPolicyRefs   []string            `json:"stream_policy_refs,omitempty"`
	SubjectPolicyRefs  []string            `json:"subject_policy_refs,omitempty"`
	OperationID        string              `json:"operation_id,omitempty"`
	ObservedGeneration int64               `json:"observed_generation,omitempty"`
}

// NATSDeployment stores registration and observed validation status.
type NATSDeployment struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Region            string              `json:"region"`
	Servers           []string            `json:"servers"`
	AccountMode       string              `json:"account_mode"`
	State             NATSDeploymentState `json:"state"`
	CredentialRef     CredentialReference `json:"credential_ref"`
	RegisteredAt      time.Time           `json:"registered_at"`
	LastValidatedAt   time.Time           `json:"last_validated_at,omitempty"`
	ObservedVersion   string              `json:"observed_version,omitempty"`
	LastError         string              `json:"last_error,omitempty"`
	OperationID       string              `json:"operation_id,omitempty"`
	StreamPolicyRefs  []string            `json:"stream_policy_refs,omitempty"`
	SubjectPolicyRefs []string            `json:"subject_policy_refs,omitempty"`
}

// NATSHealth captures the result of registration and validation.
type NATSHealth struct {
	DeploymentID  string              `json:"deployment_id"`
	State         NATSDeploymentState `json:"state"`
	ServerVersion string              `json:"server_version,omitempty"`
	Checks        []string            `json:"checks,omitempty"`
	ValidatedAt   time.Time           `json:"validated_at"`
	FailureReason string              `json:"failure_reason,omitempty"`
}

// ResourceSet lists the resource families Cloud expects to bootstrap.
type ResourceSet struct {
	Streams         []string `json:"streams,omitempty"`
	KeyValueBuckets []string `json:"key_value_buckets,omitempty"`
	ObjectStores    []string `json:"object_stores,omitempty"`
	Consumers       []string `json:"consumers,omitempty"`
}

// BackupManifest records evidence for backup and restore validation.
type BackupManifest struct {
	DeploymentID string    `json:"deployment_id"`
	PolicyRef    string    `json:"policy_ref"`
	RPOSeconds   int       `json:"rpo_seconds"`
	RTOSeconds   int       `json:"rto_seconds"`
	CreatedAt    time.Time `json:"created_at"`
	Artifacts    []string  `json:"artifacts,omitempty"`
}

// NATSDeploymentProvider is the Cloud-side contract for registration and
// bootstrap/validation against operator-hosted NATS.
type NATSDeploymentProvider interface {
	Register(spec NATSDeploymentSpec) (NATSDeployment, error)
	Validate(deployment NATSDeployment) (NATSHealth, error)
	Bootstrap(deployment NATSDeployment, resources ResourceSet) error
	Drain(deployment NATSDeployment) error
	BackupManifest(deployment NATSDeployment) (BackupManifest, error)
}

// NATSDeploymentCanTransition validates lifecycle transitions for the control
// plane contract.
func NATSDeploymentCanTransition(from, to NATSDeploymentState) bool {
	switch from {
	case NATSDeploymentUnregistered:
		return to == NATSDeploymentReady || to == NATSDeploymentUnreachable || to == NATSDeploymentIncompatible
	case NATSDeploymentReady:
		return to == NATSDeploymentDegraded || to == NATSDeploymentDraining || to == NATSDeploymentUnreachable || to == NATSDeploymentIncompatible
	case NATSDeploymentDegraded:
		return to == NATSDeploymentReady || to == NATSDeploymentDraining || to == NATSDeploymentUnreachable || to == NATSDeploymentIncompatible
	case NATSDeploymentDraining:
		return to == NATSDeploymentReady || to == NATSDeploymentUnreachable
	case NATSDeploymentUnreachable, NATSDeploymentIncompatible:
		return to == NATSDeploymentReady || to == NATSDeploymentDraining
	default:
		return false
	}
}

// NATSValidationReady reports whether validation passed compatibility,
// permission, and backup/restore gates.
func NATSValidationReady(health NATSHealth) bool {
	if health.State != NATSDeploymentReady || health.FailureReason != "" {
		return false
	}
	required := map[string]bool{
		"compatibility": false,
		"permission":    false,
		"backup":        false,
		"restore":       false,
	}
	for _, check := range health.Checks {
		if _, ok := required[check]; !ok {
			return false
		}
		required[check] = true
	}
	for _, ok := range required {
		if !ok {
			return false
		}
	}
	return true
}

// NATSValidationFailed reports whether the deployment is in a terminal
// non-ready state.
func NATSValidationFailed(state NATSDeploymentState) bool {
	return state == NATSDeploymentUnreachable || state == NATSDeploymentIncompatible
}

// NATSDeploymentIsDraining reports whether the deployment is in a fenced drain
// transition and should reject new bootstrap activity.
func NATSDeploymentIsDraining(state NATSDeploymentState) bool {
	return state == NATSDeploymentDraining
}

// NATSDeploymentNeedsRecovery reports whether the deployment is unhealthy but
// still recoverable through revalidation or rebootstrap.
func NATSDeploymentNeedsRecovery(state NATSDeploymentState) bool {
	return state == NATSDeploymentUnreachable || state == NATSDeploymentDegraded
}

// NATSDeploymentNeedsOperatorAction reports whether the deployment is in a
// terminal incompatible state.
func NATSDeploymentNeedsOperatorAction(state NATSDeploymentState) bool {
	return state == NATSDeploymentIncompatible
}
