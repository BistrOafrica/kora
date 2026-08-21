package cloud

import "time"

// BackupPolicyState tracks the lifecycle of a managed backup policy.
type BackupPolicyState string

const (
	BackupPolicyRequested BackupPolicyState = "requested"
	BackupPolicyActive    BackupPolicyState = "active"
	BackupPolicyPaused    BackupPolicyState = "paused"
	BackupPolicyFailed    BackupPolicyState = "failed"
	BackupPolicyRetired   BackupPolicyState = "retired"
)

// BackupPolicy declares the RPO/RTO and evidence requirements for a region.
type BackupPolicy struct {
	ID             string            `json:"id"`
	DeploymentID   string            `json:"deployment_id"`
	Region         string            `json:"region"`
	State          BackupPolicyState `json:"state"`
	RPOSeconds     int               `json:"rpo_seconds"`
	RTOSeconds     int               `json:"rto_seconds"`
	RetentionDays  int               `json:"retention_days"`
	RequiredChecks []string          `json:"required_checks,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at,omitempty"`
	OperationID    string            `json:"operation_id,omitempty"`
	LastError      string            `json:"last_error,omitempty"`
}

// BackupRecord captures one verified backup execution.
type BackupRecord struct {
	ID           string    `json:"id"`
	PolicyID     string    `json:"policy_id"`
	DeploymentID string    `json:"deployment_id"`
	Region       string    `json:"region"`
	ArtifactRef  string    `json:"artifact_ref"`
	VerifiedAt   time.Time `json:"verified_at"`
	CreatedAt    time.Time `json:"created_at"`
	Restorable   bool      `json:"restorable"`
}

// ObservabilitySnapshot captures operator-facing health and SLO evidence.
type ObservabilitySnapshot struct {
	ID             string    `json:"id"`
	DeploymentID   string    `json:"deployment_id"`
	Region         string    `json:"region"`
	State          string    `json:"state"`
	Healthy        bool      `json:"healthy"`
	RequestsPerMin int       `json:"requests_per_min,omitempty"`
	ErrorsPerMin   int       `json:"errors_per_min,omitempty"`
	LatencyP95Ms   int       `json:"latency_p95_ms,omitempty"`
	CollectedAt    time.Time `json:"collected_at"`
	SLORef         string    `json:"slo_ref,omitempty"`
}

// BillingUsageProjection records a tenant billing projection for a period.
type BillingUsageProjection struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	DeploymentID   string    `json:"deployment_id"`
	Region         string    `json:"region"`
	Period         string    `json:"period"`
	Currency       string    `json:"currency"`
	UsageUnits     int64     `json:"usage_units"`
	EstimatedCost  float64   `json:"estimated_cost"`
	Reconciled     bool      `json:"reconciled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

// RegionalPlacement declares the preferred and allowed regions for a
// deployment.
type RegionalPlacement struct {
	ID              string    `json:"id"`
	DeploymentID    string    `json:"deployment_id"`
	PreferredRegion string    `json:"preferred_region"`
	AllowedRegions  []string  `json:"allowed_regions,omitempty"`
	FailoverRegions []string  `json:"failover_regions,omitempty"`
	LockInRegion    bool      `json:"lock_in_region"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
}

// BackupPolicyCanTransition validates backup policy state changes.
func BackupPolicyCanTransition(from, to BackupPolicyState) bool {
	switch from {
	case BackupPolicyRequested:
		return to == BackupPolicyActive || to == BackupPolicyFailed
	case BackupPolicyActive:
		return to == BackupPolicyPaused || to == BackupPolicyRetired || to == BackupPolicyFailed
	case BackupPolicyPaused:
		return to == BackupPolicyActive || to == BackupPolicyFailed || to == BackupPolicyRetired
	case BackupPolicyFailed:
		return to == BackupPolicyRequested
	case BackupPolicyRetired:
		return false
	default:
		return false
	}
}

// BackupPolicyHealthy reports whether the policy has enough evidence to be
// treated as operational.
func BackupPolicyHealthy(policy BackupPolicy, record *BackupRecord) bool {
	if policy.State != BackupPolicyActive || policy.RPOSeconds <= 0 || policy.RTOSeconds <= 0 {
		return false
	}
	if record == nil {
		return false
	}
	return record.Restorable && record.PolicyID == policy.ID && record.Region == policy.Region
}

// ObservabilityHealthy reports whether the current snapshot is within a basic
// operational envelope.
func ObservabilityHealthy(snapshot ObservabilitySnapshot) bool {
	if !snapshot.Healthy || snapshot.State == "" {
		return false
	}
	if snapshot.ErrorsPerMin < 0 || snapshot.RequestsPerMin < 0 || snapshot.LatencyP95Ms < 0 {
		return false
	}
	return true
}

// BillingProjectionIsValid reports whether the usage projection is complete
// enough for billing reconciliation.
func BillingProjectionIsValid(proj BillingUsageProjection) bool {
	return proj.OrganizationID != "" && proj.DeploymentID != "" && proj.Region != "" && proj.Period != "" && proj.Currency != "" && proj.CreatedAt.IsZero() == false
}

// RegionalPlacementMatches reports whether a region is allowed by the
// placement policy.
func RegionalPlacementMatches(p RegionalPlacement, region string) bool {
	if p.PreferredRegion == region {
		return true
	}
	for _, candidate := range p.AllowedRegions {
		if candidate == region {
			return true
		}
	}
	for _, candidate := range p.FailoverRegions {
		if candidate == region {
			return true
		}
	}
	return false
}
