package control

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
)

var toolNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`)

const (
	ExpectedAODiscovery      = "flag_env_runfile_candidates_fail_closed"
	ExpectedGatewayDiscovery = "flag_env_historical_default_fail_closed"
)

// ParseAndValidateRunSpec strictly parses the raw bytes of a RunSpec and validates its schema and constraints.
func ParseAndValidateRunSpec(rawBytes []byte) (*RunSpec, string, error) {
	if len(rawBytes) == 0 {
		return nil, "", errors.New("run spec bytes are empty")
	}

	// Compute exact raw byte SHA-256
	h := sha256.Sum256(rawBytes)
	rawSha := hex.EncodeToString(h[:])

	// Explicit check for forbidden fields in raw JSON before unmarshaling
	var rawMap map[string]any
	if err := json.Unmarshal(rawBytes, &rawMap); err == nil {
		if _, exists := rawMap["aoUrl"]; exists {
			return nil, "", errors.New("run spec must not contain aoUrl")
		}
		if _, exists := rawMap["gatewayUrl"]; exists {
			return nil, "", errors.New("run spec must not contain gatewayUrl")
		}
		if ep, ok := rawMap["endpoints"]; ok {
			return nil, "", fmt.Errorf("run spec must not contain endpoints object: %v", ep)
		}
	}

	// Strict JSON decoding with DisallowUnknownFields
	decoder := json.NewDecoder(bytes.NewReader(rawBytes))
	decoder.DisallowUnknownFields()

	var spec RunSpec
	if err := decoder.Decode(&spec); err != nil {
		return nil, "", fmt.Errorf("strict json decode run spec: %w", err)
	}

	// Reject trailing tokens / objects after the root object
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, "", errors.New("run spec contains trailing tokens or multiple JSON objects")
	}

	// 1. Schema version
	if spec.SchemaVersion != RunSpecSchemaVersion {
		return nil, "", fmt.Errorf("invalid schemaVersion %q; expected %q", spec.SchemaVersion, RunSpecSchemaVersion)
	}

	// 2. Validate RunID
	spec.RunID = strings.TrimSpace(spec.RunID)
	if spec.RunID == "" {
		return nil, "", errors.New("runId is required")
	}

	// 3. Validate Target
	if strings.TrimSpace(spec.Target.TargetRoot) == "" {
		return nil, "", errors.New("target.targetRoot is required")
	}
	absTargetRoot, err := filepath.Abs(spec.Target.TargetRoot)
	if err != nil {
		return nil, "", fmt.Errorf("canonicalize targetRoot: %w", err)
	}
	spec.Target.TargetRoot = absTargetRoot

	if strings.TrimSpace(spec.Target.ExecutionWorkspace) == "" {
		return nil, "", errors.New("target.executionWorkspace is required")
	}
	absExecWorkspace, err := filepath.Abs(spec.Target.ExecutionWorkspace)
	if err != nil {
		return nil, "", fmt.Errorf("canonicalize executionWorkspace: %w", err)
	}
	spec.Target.ExecutionWorkspace = absExecWorkspace

	spec.Target.BaselineSha = strings.TrimSpace(spec.Target.BaselineSha)
	if spec.Target.BaselineSha == "" {
		return nil, "", errors.New("target.baselineSha is required")
	}

	spec.Target.ExpectedBranch = strings.TrimSpace(spec.Target.ExpectedBranch)
	if spec.Target.ExpectedBranch == "" {
		return nil, "", errors.New("target.expectedBranch is required")
	}

	// 4. Validate WorkspacePolicy
	if !spec.WorkspacePolicy.ProductRootMustBeClean {
		return nil, "", errors.New("workspacePolicy.productRootMustBeClean must be true")
	}
	dirtyPolicy := strings.TrimSpace(spec.WorkspacePolicy.ExecutionWorkspaceDirtyPolicy)
	if dirtyPolicy == "" {
		dirtyPolicy = "require_clean"
	}
	if dirtyPolicy != "require_clean" && dirtyPolicy != "allow_dirty_recorded" {
		return nil, "", fmt.Errorf("invalid executionWorkspaceDirtyPolicy %q; must be 'require_clean' or 'allow_dirty_recorded'", dirtyPolicy)
	}
	spec.WorkspacePolicy.ExecutionWorkspaceDirtyPolicy = dirtyPolicy

	// 5. Validate RequiredTools
	hasGit := false
	for _, tool := range spec.RequiredTools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			return nil, "", errors.New("requiredTools item has empty name")
		}
		if !toolNameRegex.MatchString(name) {
			return nil, "", fmt.Errorf("invalid tool name %q: must not contain paths or shell metacharacters", name)
		}
		if name == "git" && tool.Required {
			hasGit = true
		}
	}
	if !hasGit {
		// git is mandatory for Slice 8A
		return nil, "", errors.New("requiredTools must explicitly include git as a required tool")
	}

	// 6. Validate EndpointPolicy
	if spec.EndpointPolicy.AODiscovery != ExpectedAODiscovery {
		return nil, "", fmt.Errorf("invalid endpointPolicy.aoDiscovery %q; expected %q",
			spec.EndpointPolicy.AODiscovery, ExpectedAODiscovery)
	}
	if spec.EndpointPolicy.GatewayDiscovery != ExpectedGatewayDiscovery {
		return nil, "", fmt.Errorf("invalid endpointPolicy.gatewayDiscovery %q; expected %q",
			spec.EndpointPolicy.GatewayDiscovery, ExpectedGatewayDiscovery)
	}

	// 7. Validate Concurrency (if present)
	if spec.Concurrency != nil {
		if spec.Concurrency.MaxConcurrency < 1 || spec.Concurrency.MaxConcurrency > 3 {
			return nil, "", fmt.Errorf("concurrency.maxConcurrency %d out of bounds [1, 3]",
				spec.Concurrency.MaxConcurrency)
		}
	}

	// 8. Validate Profiles
	if len(spec.Profiles) == 0 {
		return nil, "", errors.New("profiles must contain at least one entry")
	}
	for profName, prof := range spec.Profiles {
		m := strings.TrimSpace(prof.Model)
		if m == "" {
			return nil, "", fmt.Errorf("profile %q has empty model", profName)
		}
		e := strings.TrimSpace(prof.ReasoningEffort)
		if e == "" {
			return nil, "", fmt.Errorf("profile %q has empty reasoningEffort", profName)
		}
	}

	return &spec, rawSha, nil
}
