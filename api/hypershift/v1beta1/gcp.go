package v1beta1

// GCPResourceReference represents a reference to a GCP resource by name.
// Follows GCP naming patterns (name-based APIs, not ID-based like AWS).
// See https://google.aip.dev/122 for GCP resource name standards.
type GCPResourceReference struct {
	// name is the name of the GCP resource.
	// Must conform to GCP resource naming standards: lowercase letters, numbers, and hyphens only.
	// Must start and end with lowercase letter or number, max 63 characters.
	// See https://google.aip.dev/122 for details.
	//
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`
	// +kubebuilder:validation:XValidation:rule="!self.contains('--')", message="GCP resource names cannot contain consecutive hyphens"
	Name string `json:"name"`
}

// GCPEndpointAccessType defines the endpoint access type for GCP clusters.
// Equivalent to AWS EndpointAccessType but adapted for GCP networking model.
type GCPEndpointAccessType string

const (
	// GCPEndpointAccessPublicAndPrivate endpoint access allows public API server access and
	// private node communication with the control plane via Private Service Connect.
	GCPEndpointAccessPublicAndPrivate GCPEndpointAccessType = "PublicAndPrivate"

	// GCPEndpointAccessPrivate endpoint access allows only private API server access and private
	// node communication with the control plane via Private Service Connect.
	GCPEndpointAccessPrivate GCPEndpointAccessType = "Private"
)

// GCPNetworkConfig specifies VPC configuration for GCP clusters and Private Service Connect endpoint creation.
type GCPNetworkConfig struct {
	// network is the VPC network name
	// +required
	Network GCPResourceReference `json:"network"`

	// privateServiceConnectSubnet is the subnet for Private Service Connect endpoints
	// +required
	PrivateServiceConnectSubnet GCPResourceReference `json:"privateServiceConnectSubnet"`
}

// GCPPlatformSpec specifies configuration for clusters running on Google Cloud Platform.
type GCPPlatformSpec struct {
	// project is the GCP project ID.
	// A valid project ID must satisfy the following rules:
	//   length: Must be between 6 and 30 characters, inclusive
	//   characters: Only lowercase letters (`a-z`), digits (`0-9`), and hyphens (`-`) are allowed
	//   start and end: Must begin with a lowercase letter and must not end with a hyphen
	//   hyphens: No consecutive hyphens are allowed (e.g., "my--project" is invalid)
	//   valid examples: "my-project", "my-project-1", "my-project-123".
	//
	// +required
	// +immutable
	// +kubebuilder:validation:MinLength=6
	// +kubebuilder:validation:MaxLength=30
	// +kubebuilder:validation:Pattern=`^[a-z]([a-z0-9-]{4,28}[a-z0-9])$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Project is immutable"
	// +kubebuilder:validation:XValidation:rule="!self.contains('--')", message="Project ID cannot contain consecutive hyphens"
	// +kubebuilder:validation:XValidation:rule="!self.startsWith('-') && !self.endsWith('-')", message="Project ID cannot start or end with a hyphen"
	Project string `json:"project"`

	// region is the GCP region in which the cluster resides.
	// A valid region must satisfy the following rules:
	//   format: Must be in the form `<letters>-<lettersOrDigits><digit>`
	//   characters: Only lowercase letters (`a-z`), digits (`0-9`), and a single hyphen (`-`) separator
	//   valid examples: "us-central1", "europe-west2"
	//   region must not include zone suffixes (e.g., "-a").
	// For a full list of valid regions, see: https://cloud.google.com/compute/docs/regions-zones.
	//
	// +required
	// +immutable
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z]+-[a-z0-9]+[0-9]$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Region is immutable"
	Region string `json:"region"`

	// networkConfig specifies VPC configuration for Private Service Connect.
	// Required for VPC configuration in Private Service Connect deployments.
	// +required
	NetworkConfig GCPNetworkConfig `json:"networkConfig"`

	// endpointAccess controls API endpoint accessibility for the HostedControlPlane on GCP.
	// Allowed values: "Private", "PublicAndPrivate". Defaults to "Private".
	// +kubebuilder:validation:Enum=PublicAndPrivate;Private
	// +kubebuilder:default=Private
	// +optional
	EndpointAccess GCPEndpointAccessType `json:"endpointAccess,omitempty"`

	// resourceLabels are applied to all GCP resources created for the cluster.
	// These labels help with resource organization, cost tracking, and management.
	// Keys and values must conform to GCP label requirements: keys max 63 chars, values max 63 chars.
	// Both must start with lowercase letter or number, contain only lowercase letters, numbers, underscores, hyphens.
	// Keys cannot be empty, values can be empty.
	//
	// +optional
	// +kubebuilder:validation:MaxProperties=64
	ResourceLabels map[string]string `json:"resourceLabels,omitempty"`

	// workloadIdentity configures Workload Identity Federation for the cluster.
	// This enables secure, short-lived token-based authentication without storing
	// long-term service account keys.
	// +required
	WorkloadIdentity GCPWorkloadIdentityConfig `json:"workloadIdentity"`
}

// GCPWorkloadIdentityConfig configures Workload Identity Federation for GCP clusters.
// This enables secure, short-lived token-based authentication without storing
// long-term service account keys.
type GCPWorkloadIdentityConfig struct {
	// projectNumber is the numeric GCP project identifier for WIF configuration.
	// This differs from the project ID and is required for workload identity pools.
	// Must be a numeric string (up to 20 digits).
	//
	// +required
	// +kubebuilder:validation:Pattern=`^[0-9]{1,20}$`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=20
	ProjectNumber string `json:"projectNumber"`

	// poolID is the workload identity pool identifier within the project.
	// This pool is used to manage external identity mappings.
	// Must be 4-32 characters, lowercase letters, numbers, and hyphens only.
	// Cannot start or end with a hyphen.
	//
	// +required
	// +kubebuilder:validation:MinLength=4
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]{2,30}[a-z0-9])$`
	// +kubebuilder:validation:XValidation:rule="!self.contains('--')", message="Pool ID cannot contain consecutive hyphens"
	PoolID string `json:"poolID"`

	// providerID is the workload identity provider identifier within the pool.
	// This provider handles the token exchange between external and GCP identities.
	// Must be 4-32 characters, lowercase letters, numbers, and hyphens only.
	// Cannot start or end with a hyphen.
	//
	// +required
	// +kubebuilder:validation:MinLength=4
	// +kubebuilder:validation:MaxLength=32
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]{2,30}[a-z0-9])$`
	// +kubebuilder:validation:XValidation:rule="!self.contains('--')", message="Provider ID cannot contain consecutive hyphens"
	ProviderID string `json:"providerID"`

	// serviceAccountsRef contains references to various Google Service Accounts
	// required to enable integrations for different controllers and operators.
	// This follows the AWS pattern of having different roles for different purposes.
	//
	// +required
	ServiceAccountsRef GCPServiceAccountsRef `json:"serviceAccountsRef"`
}

// GCPServiceAccountsRef contains references to Google Service Accounts for different controllers.
// Each service account should have the appropriate IAM permissions for its specific role.
type GCPServiceAccountsRef struct {
	// nodePoolEmail is the Google Service Account email for CAPG controllers
	// that manage NodePool infrastructure (VMs, networks, disks, etc.).
	// This GSA needs compute.*, network.*, and storage.* permissions.
	// Format: service-account-name@project-id.iam.gserviceaccount.com
	//
	// +required
	// +kubebuilder:validation:Pattern=`^[a-z0-9-]+@[a-z0-9-]+\.iam\.gserviceaccount\.com$`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=100
	NodePoolEmail string `json:"nodePoolEmail"`

	// +required
	// +kubebuilder:validation:Pattern=`^[a-z0-9-]+@[a-z0-9-]+\.iam\.gserviceaccount\.com$`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=100
	ControlPlaneEmail string `json:"controlPlaneEmail"`
}
