// Package gcp provides Google Cloud Platform support for HyperShift.
//
// This package implements the Platform interface to enable HyperShift
// to create and manage OpenShift control planes on Google Cloud Platform.
//
// The package currently provides enough functionality to allow HostedCluster
// resources with platform.type: GCP to be accepted and reconciled by the
// HyperShift operator without errors. All methods return nil or no-op values
// to indicate that no platform-specific operations should be performed at this time.
//
// For more information about HyperShift platform implementations, see:
// https://github.com/openshift/hypershift/blob/main/docs/content/how-to/platforms.md
package gcp

import (
	"context"
	"fmt"
	"os"
	"strings"

	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	"github.com/openshift/hypershift/support/images"
	"github.com/openshift/hypershift/support/upsert"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	capigcp "sigs.k8s.io/cluster-api-provider-gcp/api/v1beta1"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/blang/semver"
)

// GCP implements the Platform interface for Google Cloud Platform.
//
// This implementation enables HostedCluster reconciliation for GCP platform
// with CAPG (Cluster API Provider GCP) integration. The struct provides
// implementations for all Platform interface methods to support CAPG controllers,
// credential management, and Workload Identity Federation preparation.
type GCP struct {
	utilitiesImage    string
	capiProviderImage string
	payloadVersion    *semver.Version
}

// New creates a new GCP platform instance.
//
// This function returns a new instance of the GCP platform implementation
// that can be used with the HyperShift platform factory. The returned
// platform provides CAPG integration for GCP hosted clusters with proper
// image management and credential handling.
//
// Parameters:
//   - utilitiesImage: Container image for utility containers (e.g., token minter sidecar)
//   - capiProviderImage: Container image for the CAPG controller
//   - payloadVersion: OpenShift payload version for feature compatibility
//
// Returns:
//   - *GCP: A new GCP platform instance ready for use with HyperShift controllers
//
// Example usage:
//
//	platform := gcp.New("utilities-image", "capg-image", semver.MustParse("4.17.0"))
//	// platform can now be used with HyperShift platform factory
func New(utilitiesImage string, capiProviderImage string, payloadVersion *semver.Version) *GCP {
	return &GCP{
		utilitiesImage:    utilitiesImage,
		capiProviderImage: capiProviderImage,
		payloadVersion:    payloadVersion,
	}
}

// ReconcileCAPIInfraCR creates and manages the GCPCluster CAPI infrastructure resource.
// This method follows the AWS pattern for CAPI infrastructure reconciliation
// and enables CAPG controllers to manage GCP resources for NodePool support.
func (p GCP) ReconcileCAPIInfraCR(ctx context.Context, c client.Client, createOrUpdate upsert.CreateOrUpdateFN,
	hcluster *hyperv1.HostedCluster,
	controlPlaneNamespace string, apiEndpoint hyperv1.APIEndpoint) (client.Object, error) {

	// Create GCPCluster object following AWS pattern
	gcpCluster := &capigcp.GCPCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: controlPlaneNamespace,
			Name:      hcluster.Name,
		},
	}

	// Use createOrUpdate to reconcile the GCPCluster
	if _, err := createOrUpdate(ctx, c, gcpCluster, func() error {
		return p.reconcileGCPCluster(gcpCluster, hcluster, apiEndpoint)
	}); err != nil {
		return nil, fmt.Errorf("failed to reconcile GCPCluster: %w", err)
	}

	// Set Status.Ready = true following AWS/Azure patterns - CRITICAL for CAPI progression
	gcpCluster.Status.Ready = true
	if err := c.Status().Update(ctx, gcpCluster); err != nil {
		return nil, fmt.Errorf("failed to update GCPCluster status: %w", err)
	}

	return gcpCluster, nil
}

// reconcileGCPCluster configures the GCPCluster resource following AWS patterns.
// This method sets up the cluster specification with GCP-specific settings.
func (p GCP) reconcileGCPCluster(gcpCluster *capigcp.GCPCluster, hcluster *hyperv1.HostedCluster, apiEndpoint hyperv1.APIEndpoint) error {
	// Mark as managed externally (following AWS pattern)
	if gcpCluster.Annotations == nil {
		gcpCluster.Annotations = make(map[string]string)
	}
	gcpCluster.Annotations[capiv1.ManagedByAnnotation] = "external"

	// Set GCP-specific cluster configuration
	if hcluster.Spec.Platform.GCP != nil {
		gcpSpec := hcluster.Spec.Platform.GCP

		// Set project and region (required fields)
		gcpCluster.Spec.Project = gcpSpec.Project
		gcpCluster.Spec.Region = gcpSpec.Region

		// Configure network settings if available
		if gcpSpec.NetworkConfig.Network.Name != "" {
			gcpCluster.Spec.Network = capigcp.NetworkSpec{
				Name: ptr.To(gcpSpec.NetworkConfig.Network.Name),
			}

			// Set subnet for Private Service Connect if available
			if gcpSpec.NetworkConfig.PrivateServiceConnectSubnet.Name != "" {
				// CAPG handles subnets differently, configure as needed for PSC
				// This will be used by CAPG for node creation
				gcpCluster.Spec.Network.Subnets = []capigcp.SubnetSpec{
					{
						Name:   gcpSpec.NetworkConfig.PrivateServiceConnectSubnet.Name,
						Region: gcpSpec.Region,
						// CAPG will discover the network CIDR automatically
					},
				}
			}
		}

		// Add resource labels as additional labels (following AWS ResourceTags pattern)
		if len(gcpSpec.ResourceLabels) > 0 {
			if gcpCluster.Spec.AdditionalLabels == nil {
				gcpCluster.Spec.AdditionalLabels = make(map[string]string)
			}
			for key, value := range gcpSpec.ResourceLabels {
				gcpCluster.Spec.AdditionalLabels[key] = value
			}
		}

		// Add HyperShift-specific labels for resource identification
		if gcpCluster.Spec.AdditionalLabels == nil {
			gcpCluster.Spec.AdditionalLabels = make(map[string]string)
		}
		gcpCluster.Spec.AdditionalLabels[toGCPLabel("hypershift.openshift.io/cluster")] = hcluster.Name
		if hcluster.Spec.InfraID != "" {
			gcpCluster.Spec.AdditionalLabels[toGCPLabel("hypershift.openshift.io/infra-id")] = hcluster.Spec.InfraID
		}
	}

	// Set control plane endpoint (following AWS pattern)
	gcpCluster.Spec.ControlPlaneEndpoint = capiv1.APIEndpoint{
		Host: apiEndpoint.Host,
		Port: apiEndpoint.Port,
	}

	// Note: CAPG will handle failure domains and status automatically based on region
	// We don't modify Status fields directly as that's managed by CAPG controllers

	return nil
}

// CAPIProviderDeploymentSpec implements CAPG controller deployment specification.
// This method creates a deployment spec for the CAPG (Cluster API Provider GCP)
// controller with proper image handling, feature gates, and WIF preparation.
func (p GCP) CAPIProviderDeploymentSpec(hcluster *hyperv1.HostedCluster, hcp *hyperv1.HostedControlPlane) (*appsv1.DeploymentSpec, error) {
	// Image priority: annotation > env var > payload
	providerImage := p.capiProviderImage
	if envImage := os.Getenv(images.GCPCAPIProviderEnvVar); len(envImage) > 0 {
		providerImage = envImage
	}
	if override, ok := hcluster.Annotations[hyperv1.ClusterAPIGCPProviderImage]; ok {
		providerImage = override
	}

	// Validate image is available
	if providerImage == "" {
		return nil, fmt.Errorf("no GCP CAPI provider image specified")
	}

	// GCP-specific feature gates based on payload version
	featureGates := []string{
		"MachinePool=false", // Disable for Phase 1
	}

	// Version-conditional feature gates (future-proofing)
	if p.payloadVersion != nil && p.payloadVersion.Major == 4 && p.payloadVersion.Minor > 16 {
		featureGates = append(featureGates, "ClusterResourceSet=false") // Example
	}

	args := []string{
		"--namespace=$(MY_NAMESPACE)",
		"--leader-elect=true",
		fmt.Sprintf("--feature-gates=%s", strings.Join(featureGates, ",")),
		"--v=2",
	}

	containers := []corev1.Container{
		{
			Name:            "manager",
			Image:           providerImage,
			ImagePullPolicy: corev1.PullIfNotPresent, // AWS/Azure pattern
			Args:            args,
			Env: []corev1.EnvVar{
				{Name: "MY_NAMESPACE", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}}},
				{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/home/.gcp/application_default_credentials.json"},
				{Name: "GOOGLE_CLOUD_PROJECT", Value: hcluster.Spec.Platform.GCP.Project},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "credentials",
					MountPath: "/home/.gcp",
					ReadOnly:  true,
				},
				{
					Name:      "capi-webhooks-tls",
					ReadOnly:  true,
					MountPath: "/tmp/k8s-webhook-server/serving-certs",
				},
				{
					Name:      "token",
					MountPath: "/var/run/secrets/openshift/serviceaccount",
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: ptr.To(false),
				Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
				RunAsNonRoot:             ptr.To(true),
			},
		},
		p.tokenMinterSidecar(hcluster),
	}

	return &appsv1.DeploymentSpec{
		Replicas: ptr.To[int32](1),
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				TerminationGracePeriodSeconds: ptr.To[int64](10), // AWS/Azure pattern
				Containers:                    containers,
				Volumes:                       p.buildVolumes(hcluster),
				ServiceAccountName:            "capi-gcp-controller-manager",
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: ptr.To(true),
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
			},
		},
	}, nil
}

// buildVolumes creates all volumes needed for CAPG deployment including
// credentials, webhook certificates, kubeconfig, and writable token volume.
func (p GCP) buildVolumes(hcluster *hyperv1.HostedCluster) []corev1.Volume {
	defaultMode := int32(0640)
	return []corev1.Volume{
		{
			Name: "capi-webhooks-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &defaultMode,
					SecretName:  "capi-webhooks-tls",
				},
			},
		},
		{
			Name: "credentials",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: NodePoolManagementCredsSecret("").Name,
				},
			},
		},
		{
			Name: "svc-kubeconfig",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &defaultMode,
					SecretName:  "service-network-admin-kubeconfig",
				},
			},
		},
		{
			Name: "token",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory},
			},
		},
	}
}

// tokenMinterSidecar creates the token minter sidecar container.
// This sidecar generates OpenShift service account tokens that are used by the
// Google Cloud SDK for Workload Identity Federation authentication.
func (p GCP) tokenMinterSidecar(hcluster *hyperv1.HostedCluster) corev1.Container {

	wif := hcluster.Spec.Platform.GCP.WorkloadIdentity
	return corev1.Container{
		Name:    "token-minter",
		Image:   p.utilitiesImage,
		Command: []string{"/usr/bin/control-plane-operator", "token-minter"},
		Args: []string{
			"--service-account-namespace=kube-system",
			"--service-account-name=capi-gcp-controller-manager",
			"--token-audience=" + fmt.Sprintf("//iam.googleapis.com/projects/%s/locations/global/workloadIdentityPools/%s/providers/%s", wif.ProjectNumber, wif.PoolID, wif.ProviderID),
			"--token-file=/var/run/secrets/openshift/serviceaccount/token",
			"--kubeconfig=/etc/kubernetes/kubeconfig",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "token",
				MountPath: "/var/run/secrets/openshift/serviceaccount",
			},
			{
				Name:      "svc-kubeconfig",
				MountPath: "/etc/kubernetes",
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("5m"),
				corev1.ResourceMemory: resource.MustParse("32Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
			RunAsNonRoot:             ptr.To(true),
		},
	}
}

func (p GCP) ReconcileCredentials(ctx context.Context, c client.Client, createOrUpdate upsert.CreateOrUpdateFN,
	hcluster *hyperv1.HostedCluster,
	controlPlaneNamespace string) error {

	// Validate Workload Identity Federation configuration (required)
	if err := p.validateWorkloadIdentityConfiguration(hcluster); err != nil {
		setCondition(hcluster, hyperv1.ValidGCPWorkloadIdentity, metav1.ConditionFalse, "InvalidWIFConfiguration", fmt.Sprintf("Workload Identity Federation configuration is invalid: %v", err))
		return fmt.Errorf("invalid workload identity configuration: %w", err)
	}

	// Set successful WIF validation condition
	setCondition(hcluster, hyperv1.ValidGCPWorkloadIdentity, metav1.ConditionTrue, "ValidWIFConfiguration", "Workload Identity Federation configuration is valid and ready")

	// Create credential secrets following AWS pattern
	var errs []error
	syncSecret := func(secret *corev1.Secret, serviceAccountEmail string) error {
		credentials, err := buildGCPWorkloadIdentityCredentialsWithEmail(hcluster.Spec.Platform.GCP.WorkloadIdentity, serviceAccountEmail)
		if err != nil {
			return fmt.Errorf("failed to build cloud credentials secret %s/%s: %w", secret.Namespace, secret.Name, err)
		}
		if _, err := createOrUpdate(ctx, c, secret, func() error {
			secret.Data = map[string][]byte{"application_default_credentials.json": []byte(credentials)}
			secret.Type = corev1.SecretTypeOpaque
			return nil
		}); err != nil {
			return fmt.Errorf("failed to reconcile GCP cloud credential secret %s/%s: %w", secret.Namespace, secret.Name, err)
		}
		return nil
	}

	for email, secret := range map[string]*corev1.Secret{
		hcluster.Spec.Platform.GCP.WorkloadIdentity.ServiceAccountsRef.NodePoolEmail:     NodePoolManagementCredsSecret(controlPlaneNamespace),
		hcluster.Spec.Platform.GCP.WorkloadIdentity.ServiceAccountsRef.ControlPlaneEmail: ControlPlaneOperatorCredsSecret(controlPlaneNamespace),
	} {
		if err := syncSecret(secret, email); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		setCondition(hcluster, hyperv1.ValidGCPCredentials, metav1.ConditionFalse, "CredentialsError", fmt.Sprintf("Failed to reconcile credentials: %v", errs))
		return fmt.Errorf("failed to reconcile GCP credentials: %v", errs)
	}

	// Set credentials condition to indicate federation is ready
	setCondition(hcluster, hyperv1.ValidGCPCredentials, metav1.ConditionTrue, "WIFReady", "GCP Workload Identity Federation is configured and ready")

	return nil
}

// NodePoolManagementCredsSecret returns the secret containing Workload Identity Federation credentials
// for NodePool management, which is also used by CAPG following the AWS pattern.
func NodePoolManagementCredsSecret(controlPlaneNamespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: controlPlaneNamespace,
			Name:      "node-management-creds",
		},
	}
}

// ControlPlaneOperatorCredsSecret returns the secret containing Workload Identity Federation credentials
// for the control plane operator.
func ControlPlaneOperatorCredsSecret(controlPlaneNamespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: controlPlaneNamespace,
			Name:      "control-plane-operator-creds",
		},
	}
}

// buildGCPWorkloadIdentityCredentialsWithEmail creates the credential configuration for Google Cloud SDK
// to use Workload Identity Federation with a specific service account email.
func buildGCPWorkloadIdentityCredentialsWithEmail(wif hyperv1.GCPWorkloadIdentityConfig, serviceAccountEmail string) (string, error) {
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

	// Create the credential configuration that tells Google Cloud SDK how to use WIF
	// This follows the standard Google Cloud credential configuration format with service account impersonation
	// The audience must be the full resource name of the Workload Identity Provider
	credentialConfig := fmt.Sprintf(`{
  "type": "external_account",
  "audience": "//iam.googleapis.com/projects/%s/locations/global/workloadIdentityPools/%s/providers/%s",
  "subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
  "token_url": "https://sts.googleapis.com/v1/token",
  "service_account_impersonation_url": "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken",
  "credential_source": {
    "file": "/var/run/secrets/openshift/serviceaccount/token",
    "format": {
      "type": "text"
    }
  }
}`, wif.ProjectNumber, wif.PoolID, wif.ProviderID, serviceAccountEmail)

	return credentialConfig, nil
}

// buildGCPWorkloadIdentityCredentials creates the credential configuration for Google Cloud SDK
// to use Workload Identity Federation. This is equivalent to AWS's buildAWSWebIdentityCredentials.
// It uses the NodePoolEmail by default for backward compatibility.
func buildGCPWorkloadIdentityCredentials(wif hyperv1.GCPWorkloadIdentityConfig) (string, error) {
	if wif.ServiceAccountsRef.NodePoolEmail == "" {
		return "", fmt.Errorf("node pool service account email cannot be empty in GCP Workload Identity Federation credentials")
	}
	return buildGCPWorkloadIdentityCredentialsWithEmail(wif, wif.ServiceAccountsRef.NodePoolEmail)
}

// ReconcileSecretEncryption is a no-op
// TODO: Implement GCP KMS secret encryption integration.
func (p GCP) ReconcileSecretEncryption(ctx context.Context, c client.Client, createOrUpdate upsert.CreateOrUpdateFN,
	hcluster *hyperv1.HostedCluster,
	controlPlaneNamespace string) error {

	// TODO: Implement GCP KMS secret encryption
	return nil
}

// CAPIProviderPolicyRules returns RBAC policy rules required for CAPG controllers.
// Following the AWS and Azure pattern, we return nil to use standard CAPI RBAC permissions.
// GCP-specific permissions are handled through Workload Identity Federation.
func (p GCP) CAPIProviderPolicyRules() []rbacv1.PolicyRule {
	return nil
}

// DeleteCredentials is a no-op
// TODO: Implement GCP workload identity credential cleanup.
func (p GCP) DeleteCredentials(ctx context.Context, c client.Client, hcluster *hyperv1.HostedCluster, controlPlaneNamespace string) error {
	// TODO: Implement GCP credential cleanup
	return nil
}

// ValidCredentials checks if GCP credentials are valid and ready for use.
// This function validates Workload Identity Federation configuration and status.
func ValidCredentials(hc *hyperv1.HostedCluster) bool {
	// Check if GCP Workload Identity Federation is configured and valid
	validWIF := meta.FindStatusCondition(hc.Status.Conditions, string(hyperv1.ValidGCPWorkloadIdentity))
	if validWIF == nil || validWIF.Status != metav1.ConditionTrue {
		return false
	}

	// Check if GCP credentials condition indicates WIF is ready
	validCredentials := meta.FindStatusCondition(hc.Status.Conditions, string(hyperv1.ValidGCPCredentials))
	if validCredentials == nil || validCredentials.Status != metav1.ConditionTrue {
		return false
	}

	return true
}

// validateWorkloadIdentityConfiguration validates the Workload Identity Federation configuration.
// This ensures all required fields are present and properly formatted.
func (p GCP) validateWorkloadIdentityConfiguration(hcluster *hyperv1.HostedCluster) error {
	wif := hcluster.Spec.Platform.GCP.WorkloadIdentity

	// Validate project number format
	if wif.ProjectNumber == "" {
		return fmt.Errorf("project number is required")
	}
	// Additional validation is handled by OpenAPI schema

	// Validate pool ID format
	if wif.PoolID == "" {
		return fmt.Errorf("pool ID is required")
	}

	// Validate provider ID format
	if wif.ProviderID == "" {
		return fmt.Errorf("provider ID is required")
	}

	return nil
}

// toGCPLabel converts a label key to GCP-compliant format.
// GCP labels must start with a lowercase character and can only contain lowercase letters,
// numeric characters, underscores and dashes. The key can be at most 63 characters long.
// This function replaces dots and forward slashes with dashes to ensure compliance.
func toGCPLabel(label string) string {
	// Replace both dots and forward slashes with dashes for GCP compliance
	result := strings.ReplaceAll(label, ".", "-")
	result = strings.ReplaceAll(result, "/", "-")
	return result
}

// setCondition updates or creates a condition on the HostedCluster.
// This follows the standard HyperShift pattern for condition management.
func setCondition(hcluster *hyperv1.HostedCluster, conditionType hyperv1.ConditionType, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&hcluster.Status.Conditions, metav1.Condition{
		Type:               string(conditionType),
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
}
