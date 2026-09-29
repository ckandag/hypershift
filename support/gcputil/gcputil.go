package gcputil

import (
	"encoding/json"
	"fmt"

	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
)

// CredentialSource represents the credential source configuration for GCP external account credentials.
type CredentialSource struct {
	File   string                 `json:"file"`
	Format CredentialSourceFormat `json:"format"`
}

// CredentialSourceFormat represents the format of the credential source.
type CredentialSourceFormat struct {
	Type string `json:"type"`
}

// ExternalAccountCredential represents the complete GCP external account credential configuration
// for Workload Identity Federation. This follows the Google Cloud credential configuration format.
type ExternalAccountCredential struct {
	Type                           string           `json:"type"`
	Audience                       string           `json:"audience"`
	SubjectTokenType               string           `json:"subject_token_type"`
	TokenURL                       string           `json:"token_url"`
	ServiceAccountImpersonationURL string           `json:"service_account_impersonation_url"`
	CredentialSource               CredentialSource `json:"credential_source"`
}

// WorkloadIdentityAudience returns the Workload Identity Federation provider resource URI that
// scopes a GCP token to a single WIF provider. The cloud token minted for a workload must carry
// this exact value as its `aud` claim so it matches the `audience` in the external_account
// credential configuration (see BuildWorkloadIdentityCredentials) that the GCP STS token exchange
// validates. Scoping the audience to the provider URI (rather than a generic value) prevents a
// leaked token from being replayed against any other system that trusts the same OIDC issuer.
func WorkloadIdentityAudience(wif hyperv1.GCPWorkloadIdentityConfig) string {
	return fmt.Sprintf("//iam.googleapis.com/projects/%s/locations/global/workloadIdentityPools/%s/providers/%s",
		wif.ProjectNumber, wif.PoolID, wif.ProviderID)
}

// BuildWorkloadIdentityCredentials creates the credential configuration JSON for Google Cloud SDK
// to use Workload Identity Federation with a specific service account email.
func BuildWorkloadIdentityCredentials(wif hyperv1.GCPWorkloadIdentityConfig, serviceAccountEmail string) (string, error) {
	if wif.ProjectNumber == "" {
		return "", fmt.Errorf("project number cannot be empty in GCP Workload Identity Federation credentials")
	}
	if wif.PoolID == "" {
		return "", fmt.Errorf("pool ID cannot be empty in GCP Workload Identity Federation credentials")
	}
	if wif.ProviderID == "" {
		return "", fmt.Errorf("provider ID cannot be empty in GCP Workload Identity Federation credentials")
	}
	if serviceAccountEmail == "" {
		return "", fmt.Errorf("service account email cannot be empty in GCP Workload Identity Federation credentials")
	}

	credConfig := ExternalAccountCredential{
		Type:                           "external_account",
		Audience:                       WorkloadIdentityAudience(wif),
		SubjectTokenType:               "urn:ietf:params:oauth:token-type:jwt",
		TokenURL:                       "https://sts.googleapis.com/v1/token",
		ServiceAccountImpersonationURL: fmt.Sprintf("https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken", serviceAccountEmail),
		CredentialSource: CredentialSource{
			File: "/var/run/secrets/openshift/serviceaccount/token",
			Format: CredentialSourceFormat{
				Type: "text",
			},
		},
	}

	credentialJSON, err := json.Marshal(credConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal GCP credential configuration: %w", err)
	}

	return string(credentialJSON), nil
}
