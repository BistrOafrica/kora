package cloud

import "time"

// TenantQuota defines the bounded capacity Cloud may provision for one tenant
// account boundary.
type TenantQuota struct {
	MaxStreams         int   `json:"max_streams"`
	MaxConsumers       int   `json:"max_consumers"`
	MaxKeyValueBuckets int   `json:"max_key_value_buckets"`
	MaxObjectStores    int   `json:"max_object_stores"`
	MaxMessages        int   `json:"max_messages"`
	MaxStorageBytes    int64 `json:"max_storage_bytes"`
	MaxWorkers         int   `json:"max_workers"`
}

// TenantResourceSet names the initial resources Cloud validates or bootstraps
// for a tenant account boundary.
type TenantResourceSet struct {
	AccountName        string   `json:"account_name"`
	Streams            []string `json:"streams,omitempty"`
	KeyValueBuckets    []string `json:"key_value_buckets,omitempty"`
	ObjectStores       []string `json:"object_stores,omitempty"`
	EngineCredentials  []string `json:"engine_credentials,omitempty"`
	WorkerCredentials  []string `json:"worker_credentials,omitempty"`
	GatewayCredentials []string `json:"gateway_credentials,omitempty"`
}

// TenantAccount ties a tenant boundary to a registered deployment and the
// quotas used by Cloud bootstrap.
type TenantAccount struct {
	ID               string            `json:"id"`
	DeploymentID     string            `json:"deployment_id"`
	Name             string            `json:"name"`
	State            string            `json:"state"`
	Quota            TenantQuota       `json:"quota"`
	Resources        TenantResourceSet `json:"resources"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at,omitempty"`
	OperationID      string            `json:"operation_id,omitempty"`
	LastBootstrapAt  time.Time         `json:"last_bootstrap_at,omitempty"`
	LastValidationAt time.Time         `json:"last_validation_at,omitempty"`
	LastError        string            `json:"last_error,omitempty"`
}

// TenantBootstrapPlan records the durable, resumable bootstrap intent for a
// tenant boundary.
type TenantBootstrapPlan struct {
	TenantID       string            `json:"tenant_id"`
	DeploymentID   string            `json:"deployment_id"`
	Resources      TenantResourceSet `json:"resources"`
	Quota          TenantQuota       `json:"quota"`
	RequestedAt    time.Time         `json:"requested_at"`
	OperationID    string            `json:"operation_id,omitempty"`
	Resumable      bool              `json:"resumable"`
	ValidationOnly bool              `json:"validation_only"`
}

// TenantBootstrapState tracks high-level bootstrap lifecycle.
type TenantBootstrapState string

const (
	TenantBootstrapRequested    TenantBootstrapState = "requested"
	TenantBootstrapValidating   TenantBootstrapState = "validating"
	TenantBootstrapProvisioning TenantBootstrapState = "provisioning"
	TenantBootstrapReady        TenantBootstrapState = "ready"
	TenantBootstrapFailed       TenantBootstrapState = "failed"
)

// TenantBootstrapCanTransition validates state movement for a bootstrap job.
func TenantBootstrapCanTransition(from, to TenantBootstrapState) bool {
	switch from {
	case TenantBootstrapRequested:
		return to == TenantBootstrapValidating || to == TenantBootstrapFailed
	case TenantBootstrapValidating:
		return to == TenantBootstrapProvisioning || to == TenantBootstrapFailed
	case TenantBootstrapProvisioning:
		return to == TenantBootstrapReady || to == TenantBootstrapFailed
	case TenantBootstrapReady:
		return false
	case TenantBootstrapFailed:
		return to == TenantBootstrapRequested
	default:
		return false
	}
}
