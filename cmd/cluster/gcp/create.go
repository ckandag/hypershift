package gcp

import (
	"context"
	"fmt"
	"regexp"

	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	"github.com/openshift/hypershift/cmd/cluster/core"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var _ core.Platform = (*CreateOptions)(nil)

// RawCreateOptions contains the raw command-line options for creating a GCP cluster
type RawCreateOptions struct {
	// Project is the GCP project ID where the HostedCluster will be created
	Project string

	// Region is the GCP region where the HostedCluster will be created
	Region string
}

// BindOptions binds the GCP-specific flags to the provided flag set
func BindOptions(opts *RawCreateOptions, flags *pflag.FlagSet) {
	flags.StringVar(&opts.Project, "project", opts.Project, "GCP project ID where the HostedCluster will be created")
	flags.StringVar(&opts.Region, "region", opts.Region, "GCP region where the HostedCluster will be created")
}

// ValidatedCreateOptions represents validated options for creating a GCP cluster
type ValidatedCreateOptions struct {
	// Embed a private pointer that cannot be instantiated outside of this package.
	*validatedCreateOptions
}

// validatedCreateOptions is a private wrapper that enforces a call of Validate() before Complete() can be invoked.
type validatedCreateOptions struct {
	*RawCreateOptions
}

// Validate validates the GCP create cluster command options
func (o *RawCreateOptions) Validate(_ context.Context, _ *core.CreateOptions) (core.PlatformCompleter, error) {
	// Validate GCP project ID format
	if err := validateProjectID(o.Project); err != nil {
		return nil, fmt.Errorf("invalid project ID: %w", err)
	}

	// Validate GCP region format
	if err := validateRegion(o.Region); err != nil {
		return nil, fmt.Errorf("invalid region: %w", err)
	}

	return &ValidatedCreateOptions{
		validatedCreateOptions: &validatedCreateOptions{
			RawCreateOptions: o,
		},
	}, nil
}

// validateProjectID validates the GCP project ID format
// Project IDs must be between 6 and 30 characters, contain only lowercase letters,
// digits, and hyphens, start with a lowercase letter, and not end with a hyphen
func validateProjectID(projectID string) error {
	if len(projectID) < 6 || len(projectID) > 30 {
		return fmt.Errorf("project ID must be between 6 and 30 characters")
	}

	matched, err := regexp.MatchString(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`, projectID)
	if err != nil {
		return fmt.Errorf("error validating project ID: %w", err)
	}
	if !matched {
		return fmt.Errorf("project ID must start with a lowercase letter, contain only lowercase letters, digits, and hyphens, and not end with a hyphen")
	}

	return nil
}

// validateRegion validates the GCP region format
// Regions must be in the format <letters>-<lettersOrDigits><digit>
func validateRegion(region string) error {
	if len(region) == 0 {
		return fmt.Errorf("region cannot be empty")
	}

	if len(region) > 63 {
		return fmt.Errorf("region must be at most 63 characters")
	}

	matched, err := regexp.MatchString(`^[a-z]+-[a-z0-9]+[0-9]$`, region)
	if err != nil {
		return fmt.Errorf("error validating region: %w", err)
	}
	if !matched {
		return fmt.Errorf("region must be in the format <letters>-<lettersOrDigits><digit> (e.g., us-central1, europe-west2)")
	}

	return nil
}

// completedCreateOptions is a private wrapper that enforces a call of Complete() before cluster creation can be invoked.
type completedCreateOptions struct {
	*ValidatedCreateOptions

	externalDNSDomain string
	name, namespace   string
}

// CreateOptions represents the completed and validated options for creating a GCP cluster
type CreateOptions struct {
	// Embed a private pointer that cannot be instantiated outside of this package.
	*completedCreateOptions
}

// Complete completes the GCP create cluster command options
func (o *ValidatedCreateOptions) Complete(ctx context.Context, opts *core.CreateOptions) (core.Platform, error) {
	return &CreateOptions{
		completedCreateOptions: &completedCreateOptions{
			ValidatedCreateOptions: o,
			name:                   opts.Name,
			namespace:              opts.Namespace,
			externalDNSDomain:      opts.ExternalDNSDomain,
		},
	}, nil
}

// DefaultOptions returns default options for GCP cluster creation
func DefaultOptions() *RawCreateOptions {
	return &RawCreateOptions{}
}

// NewCreateCommand creates a new cobra command for creating GCP clusters
func NewCreateCommand(opts *core.RawCreateOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "gcp",
		Short:        "Creates basic functional HostedCluster resources on GCP",
		SilenceUsage: true,
	}

	gcpOpts := DefaultOptions()
	BindOptions(gcpOpts, cmd.Flags())
	_ = cmd.MarkPersistentFlagRequired("project")
	_ = cmd.MarkPersistentFlagRequired("region")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if opts.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
			defer cancel()
		}

		if err := core.CreateCluster(ctx, opts, gcpOpts); err != nil {
			opts.Log.Error(err, "Failed to create cluster")
			return err
		}
		return nil
	}

	return cmd
}

// ApplyPlatformSpecifics applies GCP-specific configurations to the HostedCluster
func (o *CreateOptions) ApplyPlatformSpecifics(hostedCluster *hyperv1.HostedCluster) error {
	hostedCluster.Spec.Platform.Type = hyperv1.GCPPlatform
	hostedCluster.Spec.Platform.GCP = &hyperv1.GCPPlatformSpec{
		Project: o.Project,
		Region:  o.Region,
	}
	return nil
}

// GenerateNodePools generates the NodePool resources for GCP
func (o *CreateOptions) GenerateNodePools(constructor core.DefaultNodePoolConstructor) []*hyperv1.NodePool {
	nodePool := constructor(hyperv1.GCPPlatform, "")
	return []*hyperv1.NodePool{nodePool}
}

// GenerateResources generates additional resources for GCP
func (o *CreateOptions) GenerateResources() ([]client.Object, error) {
	return nil, nil
}
