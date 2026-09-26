package control

import (
	"github.com/trungqwe/agents-coworkers/internal/recovery"
)

// Standard exit codes for coworkers CLI
const (
	ExitCodeSuccess             = 0
	ExitCodeUsageOrUnsupported  = 1
	ExitCodeValidation          = 2
	ExitCodeEndpointUnavailable = 3
)

// Typed error category for discovery and probes
type DiscoveryErrorKind int

const (
	ErrKindValidation  DiscoveryErrorKind = ExitCodeValidation
	ErrKindUnavailable DiscoveryErrorKind = ExitCodeEndpointUnavailable
)

type DiscoveryError struct {
	Kind DiscoveryErrorKind
	Err  error
}

func (e *DiscoveryError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *DiscoveryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// RunSpec Schema
const RunSpecSchemaVersion = "run-spec/v1-draft"

type RunSpec struct {
	SchemaVersion   string                 `json:"schemaVersion"`
	RunID           string                 `json:"runId"`
	Description     string                 `json:"description,omitempty"`
	Target          TargetSpec             `json:"target"`
	WorkspacePolicy WorkspacePolicy        `json:"workspacePolicy"`
	RequiredTools   []RequiredTool         `json:"requiredTools"`
	EndpointPolicy  EndpointPolicy         `json:"endpointPolicy"`
	Concurrency     *ConcurrencySpec       `json:"concurrency,omitempty"`
	Profiles        map[string]ProfileSpec `json:"profiles"`
	Documentation   map[string][]string    `json:"documentation,omitempty"`
	Policies        map[string]string      `json:"policies,omitempty"`
}

type TargetSpec struct {
	TargetRoot         string `json:"targetRoot"`
	ExecutionWorkspace string `json:"executionWorkspace"`
	BaselineSha        string `json:"baselineSha"`
	ExpectedBranch     string `json:"expectedBranch"`
}

type WorkspacePolicy struct {
	ProductRootMustBeClean        bool   `json:"productRootMustBeClean"`
	ExecutionWorkspaceDirtyPolicy string `json:"executionWorkspaceDirtyPolicy"` // "require_clean" or "allow_dirty_recorded"
}

type RequiredTool struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
}

type EndpointPolicy struct {
	AODiscovery      string `json:"aoDiscovery"`
	GatewayDiscovery string `json:"gatewayDiscovery"`
}

type ConcurrencySpec struct {
	MaxConcurrency int `json:"maxConcurrency"`
}

type ProfileSpec struct {
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoningEffort"`
}

// RunManifest Schema
const RunManifestSchemaVersion = "run-manifest/v1-draft"

type RunManifest struct {
	SchemaVersion          string                  `json:"schemaVersion"`
	RunID                  string                  `json:"runId"`
	RunSpecSha256          string                  `json:"runSpecSha256"`
	GeneratedAt            string                  `json:"generatedAt"`
	GeneratedBy            string                  `json:"generatedBy"` // "coworkers attach"
	Target                 TargetManifest          `json:"target"`
	Endpoints              EndpointsManifest       `json:"endpoints"`
	SessionBinding         SessionBindingManifest  `json:"sessionBinding"`
	WorktreeBinding        WorktreeBindingManifest `json:"worktreeBinding"`
	AttachedSessionProfile AttachedSessionProfile  `json:"attachedSessionProfile"`
	Lease                  LeaseManifest           `json:"lease"`
	SourceProvenance       SourceProvenance        `json:"sourceProvenance"`
}

type TargetManifest struct {
	TargetRoot         string `json:"targetRoot"`
	ExecutionWorkspace string `json:"executionWorkspace"`
	BaselineSha        string `json:"baselineSha"`
	CurrentHead        string `json:"currentHead"`
	Branch             string `json:"branch"`
	RepositoryIdentity string `json:"repositoryIdentity"`
}

type EndpointsManifest struct {
	AOURL             string               `json:"aoUrl"`
	AOIdentity        string               `json:"aoIdentity"`
	AODiscoverySource string               `json:"aoDiscoverySource"` // "run_file", "explicit_url", "environment_url"
	AOPid             int                  `json:"aoPid"`
	AOPidStatus       string               `json:"aoPidStatus"` // "VERIFIED", "NOT_OBSERVED"
	GatewayProbe      GatewayProbeManifest `json:"gatewayProbe"`
}

type GatewayProbeManifest struct {
	URL                   string   `json:"url"`
	CatalogPath           string   `json:"catalogPath"`   // "/v1/models"
	CatalogStatus         string   `json:"catalogStatus"` // "VERIFIED"
	ObservedModels        []string `json:"observedModels"`
	ProviderCallPerformed bool     `json:"providerCallPerformed"` // false
	CredentialEligibility string   `json:"credentialEligibility"` // "NOT_OBSERVED"
}

type SessionBindingManifest struct {
	SessionID     string `json:"sessionId"`
	ProjectID     string `json:"projectId"`
	SessionBranch string `json:"sessionBranch"`
}

type WorktreeBindingManifest struct {
	CanonicalPath     string `json:"canonicalPath"`
	WorktreeBranch    string `json:"worktreeBranch"`
	Head              string `json:"head"`
	IsClean           bool   `json:"isClean"`
	DirtyPolicy       string `json:"dirtyPolicy"`
	PorcelainSha256   string `json:"porcelainSha256"`
	VerifiedPorcelain bool   `json:"verifiedPorcelain"`
}

type AttachedSessionProfile struct {
	Kind            string `json:"kind"`
	Harness         string `json:"harness"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoningEffort"`
	Status          string `json:"status"` // "VERIFIED"
}

type LeaseManifest struct {
	WorkspaceRoot string              `json:"workspaceRoot"`
	RunOwner      string              `json:"runOwner"`
	TaskID        string              `json:"taskId"` // "__run__"
	OwnerID       string              `json:"ownerId"`
	ObservedState recovery.LeaseState `json:"observedState"`
	ObservedPID   int                 `json:"observedPid"`
}

type SourceProvenance struct {
	CLIProxyAPISha  string `json:"cliProxyApiSha"`
	SourceAuditOnly bool   `json:"sourceAuditOnly"`
}

// DoctorReport Schema
type DoctorReport struct {
	Status          string            `json:"status"` // "PASS" or "FAIL"
	RunID           string            `json:"runId"`
	RunSpecSha256   string            `json:"runSpecSha256"`
	AOEndpoint      AODiscoveryResult `json:"aoEndpoint"`
	GatewayEndpoint GatewayResult     `json:"gatewayEndpoint"`
	TargetRootClean bool              `json:"targetRootClean"`
	WorkspaceCheck  WorkspaceCheck    `json:"workspaceCheck"`
	BaselineFound   bool              `json:"baselineFound"`
	BranchMatch     bool              `json:"branchMatch"`
	ToolsCheck      []ToolCheckResult `json:"toolsCheck"`
	ModelPresence   map[string]bool   `json:"modelPresence"`
	Errors          []string          `json:"errors,omitempty"`
}

type AODiscoveryResult struct {
	URL             string `json:"url"`
	Identity        string `json:"identity"`
	DiscoverySource string `json:"discoverySource"`
	PID             int    `json:"pid"`
	PIDStatus       string `json:"pidStatus"`
}

type GatewayResult struct {
	URL                   string   `json:"url"`
	CatalogStatus         string   `json:"catalogStatus"`
	ObservedModels        []string `json:"observedModels"`
	ProviderCallPerformed bool     `json:"providerCallPerformed"`
	CredentialEligibility string   `json:"credentialEligibility"`
}

type WorkspaceCheck struct {
	Policy          string `json:"policy"`
	IsClean         bool   `json:"isClean"`
	PorcelainSha256 string `json:"porcelainSha256"`
}

type ToolCheckResult struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Found    bool   `json:"found"`
	Path     string `json:"path,omitempty"`
}
