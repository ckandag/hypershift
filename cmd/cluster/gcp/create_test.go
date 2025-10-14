package gcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	"github.com/openshift/hypershift/cmd/cluster/core"
	"github.com/openshift/hypershift/support/certs"
	"github.com/openshift/hypershift/support/testutil"
	"github.com/openshift/hypershift/test/integration/framework"

	utilrand "k8s.io/apimachinery/pkg/util/rand"

	"github.com/spf13/pflag"
)

func TestValidateProjectID(t *testing.T) {
	tests := map[string]struct {
		projectID     string
		expectedError bool
	}{
		"valid project ID": {
			projectID:     "my-project-123",
			expectedError: false,
		},
		"valid project ID with minimal length": {
			projectID:     "proj12",
			expectedError: false,
		},
		"valid project ID with maximal length": {
			projectID:     "abcdefghijklmnopqrstuvwxyz1234",
			expectedError: false,
		},
		"too short project ID": {
			projectID:     "short",
			expectedError: true,
		},
		"too long project ID": {
			projectID:     "this-project-id-is-way-too-long-to-be-valid",
			expectedError: true,
		},
		"project ID starting with digit": {
			projectID:     "123project",
			expectedError: true,
		},
		"project ID starting with hyphen": {
			projectID:     "-project",
			expectedError: true,
		},
		"project ID ending with hyphen": {
			projectID:     "project-",
			expectedError: true,
		},
		"project ID with uppercase letters": {
			projectID:     "My-Project",
			expectedError: true,
		},
		"project ID with underscores": {
			projectID:     "my_project",
			expectedError: true,
		},
		"project ID with consecutive hyphens": {
			projectID:     "my--project",
			expectedError: false,
		},
		"empty project ID": {
			projectID:     "",
			expectedError: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			err := validateProjectID(test.projectID)
			if test.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestValidateRegion(t *testing.T) {
	tests := map[string]struct {
		region        string
		expectedError bool
	}{
		"valid region us-central1": {
			region:        "us-central1",
			expectedError: false,
		},
		"valid region europe-west2": {
			region:        "europe-west2",
			expectedError: false,
		},
		"valid region asia-southeast1": {
			region:        "asia-southeast1",
			expectedError: false,
		},
		"valid region with numbers": {
			region:        "us-west1",
			expectedError: false,
		},
		"empty region": {
			region:        "",
			expectedError: true,
		},
		"region without hyphen": {
			region:        "uscentral1",
			expectedError: true,
		},
		"region not ending with digit": {
			region:        "us-central",
			expectedError: true,
		},
		"region starting with digit": {
			region:        "1us-central1",
			expectedError: true,
		},
		"region with uppercase": {
			region:        "US-central1",
			expectedError: true,
		},
		"region too long": {
			region:        "a-very-long-region-name-that-exceeds-the-maximum-allowed-length",
			expectedError: true,
		},
		"region with multiple hyphens": {
			region:        "us-central-west1",
			expectedError: true,
		},
		"region ending with letter": {
			region:        "us-central1a",
			expectedError: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			err := validateRegion(test.region)
			if test.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestRawCreateOptionsValidate(t *testing.T) {
	tests := map[string]struct {
		opts          *RawCreateOptions
		expectedError bool
	}{
		"valid options": {
			opts: &RawCreateOptions{
				Project: "my-project-123",
				Region:  "us-central1",
			},
			expectedError: false,
		},
		"invalid project ID": {
			opts: &RawCreateOptions{
				Project: "bad_project",
				Region:  "us-central1",
			},
			expectedError: true,
		},
		"invalid region": {
			opts: &RawCreateOptions{
				Project: "my-project-123",
				Region:  "invalid-region",
			},
			expectedError: true,
		},
		"empty project ID": {
			opts: &RawCreateOptions{
				Project: "",
				Region:  "us-central1",
			},
			expectedError: true,
		},
		"empty region": {
			opts: &RawCreateOptions{
				Project: "my-project-123",
				Region:  "",
			},
			expectedError: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			ctx := context.Background()
			coreOpts := &core.CreateOptions{}
			
			_, err := test.opts.Validate(ctx, coreOpts)
			if test.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).To(BeNil())
			}
		})
	}
}

func TestCreateOptionsApplyPlatformSpecifics(t *testing.T) {
	g := NewGomegaWithT(t)
	
	opts := &CreateOptions{
		completedCreateOptions: &completedCreateOptions{
			ValidatedCreateOptions: &ValidatedCreateOptions{
				validatedCreateOptions: &validatedCreateOptions{
					RawCreateOptions: &RawCreateOptions{
						Project: "test-project-123",
						Region:  "us-central1",
					},
				},
			},
		},
	}
	
	hostedCluster := &hyperv1.HostedCluster{}
	
	err := opts.ApplyPlatformSpecifics(hostedCluster)
	g.Expect(err).To(BeNil())
	g.Expect(hostedCluster.Spec.Platform.Type).To(Equal(hyperv1.GCPPlatform))
	g.Expect(hostedCluster.Spec.Platform.GCP).ToNot(BeNil())
	g.Expect(hostedCluster.Spec.Platform.GCP.Project).To(Equal("test-project-123"))
	g.Expect(hostedCluster.Spec.Platform.GCP.Region).To(Equal("us-central1"))
}

func TestGenerateNodePools(t *testing.T) {
	g := NewGomegaWithT(t)
	
	opts := &CreateOptions{}
	
	constructor := func(platformType hyperv1.PlatformType, arch string) *hyperv1.NodePool {
		return &hyperv1.NodePool{
			Spec: hyperv1.NodePoolSpec{
				Platform: hyperv1.NodePoolPlatform{
					Type: platformType,
				},
			},
		}
	}
	
	nodePools := opts.GenerateNodePools(constructor)
	g.Expect(nodePools).To(HaveLen(1))
	g.Expect(nodePools[0].Spec.Platform.Type).To(Equal(hyperv1.GCPPlatform))
}

func TestGenerateResources(t *testing.T) {
	g := NewGomegaWithT(t)
	
	opts := &CreateOptions{}
	
	resources, err := opts.GenerateResources()
	g.Expect(err).To(BeNil())
	g.Expect(resources).To(BeNil())
}

func TestDefaultOptions(t *testing.T) {
	g := NewGomegaWithT(t)
	
	opts := DefaultOptions()
	g.Expect(opts).ToNot(BeNil())
	g.Expect(opts.Project).To(Equal(""))
	g.Expect(opts.Region).To(Equal(""))
}

func TestCreateCluster(t *testing.T) {
	utilrand.Seed(1234567890)
	certs.UnsafeSeed(1234567890)
	ctx := framework.InterruptableContext(t.Context())
	tempDir := t.TempDir()
	t.Setenv("FAKE_CLIENT", "true")

	pullSecretFile := filepath.Join(tempDir, "pull-secret.json")
	if err := os.WriteFile(pullSecretFile, []byte(`fake`), 0600); err != nil {
		t.Fatalf("failed to write pullSecret: %v", err)
	}

	for _, testCase := range []struct {
		name string
		args []string
	}{
		{
			name: "minimal flags necessary to render",
			args: []string{
				"--project=test-project-123",
				"--region=us-central1",
				"--render-sensitive",
				"--name=example",
				"--pull-secret=" + pullSecretFile,
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			flags := pflag.NewFlagSet(testCase.name, pflag.ContinueOnError)
			coreOpts := core.DefaultOptions()
			core.BindDeveloperOptions(coreOpts, flags)
			gcpOpts := DefaultOptions()
			BindOptions(gcpOpts, flags)
			if err := flags.Parse(testCase.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			tempDir := t.TempDir()
			manifestsFile := filepath.Join(tempDir, "manifests.yaml")
			coreOpts.Render = true
			coreOpts.RenderInto = manifestsFile

			if err := core.CreateCluster(ctx, coreOpts, gcpOpts); err != nil {
				t.Fatalf("failed to create cluster: %v", err)
			}

			manifests, err := os.ReadFile(manifestsFile)
			if err != nil {
				t.Fatalf("failed to read manifests file: %v", err)
			}
			testutil.CompareWithFixture(t, manifests)
		})
	}
}