package cloud

import (
	"testing"
	"time"
)

type managedNATSProvider struct{}

func (managedNATSProvider) Register(spec NATSDeploymentSpec) (NATSDeployment, error) {
	return NATSDeployment{ID: spec.ID, Name: spec.Name, Region: spec.Region, Servers: spec.Servers, AccountMode: spec.AccountMode, State: NATSDeploymentReady, CredentialRef: spec.CredentialRef, RegisteredAt: time.Now().UTC()}, nil
}

func (managedNATSProvider) Validate(deployment NATSDeployment) (NATSHealth, error) {
	return NATSHealth{DeploymentID: deployment.ID, State: NATSDeploymentReady, Checks: []string{"compatibility", "permission", "backup", "restore"}, ValidatedAt: time.Now().UTC()}, nil
}

func (managedNATSProvider) Bootstrap(deployment NATSDeployment, resources ResourceSet) error {
	return nil
}

func (managedNATSProvider) Drain(deployment NATSDeployment) error {
	return nil
}

func (managedNATSProvider) BackupManifest(deployment NATSDeployment) (BackupManifest, error) {
	return BackupManifest{DeploymentID: deployment.ID, CreatedAt: time.Now().UTC()}, nil
}

func TestManagedNATSProviderUsesSameContract(t *testing.T) {
	var _ NATSDeploymentProvider = managedNATSProvider{}

	provider := managedNATSProvider{}
	deployment, err := provider.Register(NATSDeploymentSpec{
		ID:            "nats-1",
		Name:          "managed-nats",
		Region:        "af-south",
		Servers:       []string{"tls://nats.example.internal:4222"},
		AccountMode:   "per_tenant",
		CredentialRef: CredentialReference{ID: "cred-1", Provider: "nats", SecretRef: "secret://nats/managed"},
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	health, err := provider.Validate(deployment)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !NATSValidationReady(health) {
		t.Fatal("managed provider should emit the same validation contract")
	}
	manifest, err := provider.BackupManifest(deployment)
	if err != nil {
		t.Fatalf("backup manifest: %v", err)
	}
	if manifest.DeploymentID != deployment.ID {
		t.Fatal("backup manifest should be tied to the same deployment contract")
	}
}
