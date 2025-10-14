# GCP-120: Add GCP Platform Support to Hypershift CLI

## Problem Statement

The Hypershift CLI currently supports multiple cloud platforms (AWS, Azure, PowerVS, OpenStack, KubeVirt, Agent, None) but lacks support for Google Cloud Platform (GCP). Users need to be able to create hosted clusters on GCP through the CLI using `hypershift create cluster gcp` command.

## Context

- The GCP platform specification already exists in the Hypershift API (`api/hypershift/v1beta1/gcp.go`)
- The GCP platform spec defines two required fields: `project` and `region`
- Other platform implementations follow a consistent pattern in the codebase
- The CLI structure is well-established with clear patterns for adding new platforms

## Acceptance Criteria

1. Implement `hypershift create cluster gcp` command
2. Support `--project` flag for GCP project ID
3. Support `--region` flag for GCP region
4. Include all global flags from the core create options
5. Follow existing platform implementation patterns
6. Validate project ID and region format according to GCP requirements

## Implementation Plan

### 1. Create GCP Platform Module
- Create `cmd/cluster/gcp/` directory structure
- Implement `create.go` with GCP-specific options and command
- Follow patterns from existing platforms (AWS, Azure, etc.)

### 2. Core Components to Implement

#### 2.1 GCP Raw Options Structure
```go
type RawCreateOptions struct {
    Project string
    Region  string
    // Additional GCP-specific options as needed
}
```

#### 2.2 GCP Create Options
```go
type CreateOptions struct {
    core.CreateOptions
    Project string
    Region  string
}
```

#### 2.3 Command Registration
- Add GCP command to `cmd/cluster/cluster.go`
- Register both create and destroy commands

#### 2.4 Platform Interface Implementation
- Implement `core.Platform` interface
- Handle platform-specific validation
- Set up GCP platform spec in HostedCluster

### 3. Validation Logic
- Validate GCP project ID format (6-30 chars, lowercase letters, digits, hyphens)
- Validate GCP region format (e.g., "us-central1", "europe-west2")
- Implement field validation according to API constraints

### 4. Integration Points
- Update `cmd/cluster/cluster.go` to include GCP commands
- Ensure proper integration with core create/destroy flows
- Add GCP import to cluster package

### 5. Testing
- Create comprehensive unit tests
- Test command parsing and validation
- Test platform spec creation
- Test integration with core functionality

## File Structure
```
cmd/cluster/gcp/
├── create.go          # Main GCP create command implementation
├── create_test.go     # Unit tests
└── types.go           # Type definitions (if needed)
```

## Dependencies
- Existing GCP platform API types (`api/hypershift/v1beta1/gcp.go`)
- Core create functionality (`cmd/cluster/core/`)
- Standard cobra/pflag libraries for CLI

## Technical Considerations

1. **Field Validation**: Use kubebuilder validation patterns from API types
2. **Error Handling**: Follow existing error handling patterns from other platforms
3. **Documentation**: Ensure proper godoc comments for public functions
4. **Code Style**: Match existing code conventions and formatting

## Non-Goals
- GCP infrastructure provisioning (out of scope for this CLI feature)
- GCP-specific advanced configuration options beyond project and region
- GCP authentication handling (handled elsewhere in the stack)

## Success Metrics
- CLI command executes successfully: `hypershift create cluster gcp --project my-project --region us-central1`
- Proper validation of project and region parameters
- Generated HostedCluster resource contains correct GCP platform specification
- Unit tests achieve good coverage
- Documentation is clear and follows existing patterns