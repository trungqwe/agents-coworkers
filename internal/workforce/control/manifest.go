package control

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/trungqwe/agents-coworkers/internal/recovery"
)

func is64Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func isCleanAbsPath(p string) bool {
	if strings.TrimSpace(p) == "" {
		return false
	}
	return filepath.IsAbs(p) && filepath.Clean(p) == p
}

func is40Hex(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// ReadManifest strictly reads and validates a RunManifest from disk.
// Uses json.Decoder with DisallowUnknownFields and verifies EOF.
func ReadManifest(path string) (*RunManifest, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("manifest path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest file: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var m RunManifest
	if err := decoder.Decode(&m); err != nil {
		return nil, fmt.Errorf("strict decode manifest json: %w", err)
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("manifest contains trailing tokens or multiple JSON objects")
	}

	if m.SchemaVersion != RunManifestSchemaVersion {
		return nil, fmt.Errorf("invalid manifest schemaVersion %q; expected %q", m.SchemaVersion, RunManifestSchemaVersion)
	}
	if strings.TrimSpace(m.RunID) == "" {
		return nil, errors.New("manifest missing runId")
	}
	if !is64Hex(m.RunSpecSha256) {
		return nil, fmt.Errorf("manifest runSpecSha256 %q is not a valid 64-hex sha256", m.RunSpecSha256)
	}

	// 1. generatedAt must be valid RFC3339
	if _, err := time.Parse(time.RFC3339, m.GeneratedAt); err != nil {
		return nil, fmt.Errorf("manifest generatedAt %q is not valid RFC3339: %w", m.GeneratedAt, err)
	}

	if m.GeneratedBy != "coworkers attach" {
		return nil, fmt.Errorf("invalid generatedBy %q; expected 'coworkers attach'", m.GeneratedBy)
	}

	// 2. Canonical path validation for all stored paths:
	// targetRoot, executionWorkspace, repositoryIdentity, worktreeBinding.canonicalPath, lease.workspaceRoot
	// Must satisfy: filepath.IsAbs(path) == true && filepath.Clean(path) == path
	if !isCleanAbsPath(m.Target.TargetRoot) {
		return nil, fmt.Errorf("manifest target.targetRoot %q is not a clean absolute path", m.Target.TargetRoot)
	}
	if !isCleanAbsPath(m.Target.ExecutionWorkspace) {
		return nil, fmt.Errorf("manifest target.executionWorkspace %q is not a clean absolute path", m.Target.ExecutionWorkspace)
	}
	if !isCleanAbsPath(m.Target.RepositoryIdentity) {
		return nil, fmt.Errorf("manifest target.repositoryIdentity %q is not a clean absolute path", m.Target.RepositoryIdentity)
	}
	if !isCleanAbsPath(m.WorktreeBinding.CanonicalPath) {
		return nil, fmt.Errorf("manifest worktreeBinding.canonicalPath %q is not a clean absolute path", m.WorktreeBinding.CanonicalPath)
	}
	if !isCleanAbsPath(m.Lease.WorkspaceRoot) {
		return nil, fmt.Errorf("manifest lease.workspaceRoot %q is not a clean absolute path", m.Lease.WorkspaceRoot)
	}

	if strings.TrimSpace(m.Target.BaselineSha) == "" || strings.TrimSpace(m.Target.CurrentHead) == "" {
		return nil, errors.New("manifest missing target baselineSha or currentHead")
	}
	if strings.TrimSpace(m.Target.Branch) == "" {
		return nil, errors.New("manifest missing target branch")
	}

	// 3. Worktree binding path must match target.executionWorkspace
	if !arePathsSameOrEquivalent(m.WorktreeBinding.CanonicalPath, m.Target.ExecutionWorkspace) {
		return nil, errors.New("manifest worktreeBinding.canonicalPath does not match target.executionWorkspace")
	}

	// 4. Lease workspaceRoot must match target.executionWorkspace
	if !arePathsSameOrEquivalent(m.Lease.WorkspaceRoot, m.Target.ExecutionWorkspace) {
		return nil, errors.New("manifest lease.workspaceRoot does not match target.executionWorkspace")
	}

	// 5. Session invariants
	if strings.TrimSpace(m.SessionBinding.SessionID) == "" || strings.TrimSpace(m.SessionBinding.ProjectID) == "" || strings.TrimSpace(m.SessionBinding.SessionBranch) == "" {
		return nil, errors.New("manifest missing session binding fields")
	}

	// 6. Worktree invariants
	if strings.TrimSpace(m.WorktreeBinding.WorktreeBranch) == "" || strings.TrimSpace(m.WorktreeBinding.Head) == "" {
		return nil, errors.New("manifest missing worktree binding fields")
	}
	if !m.WorktreeBinding.VerifiedPorcelain {
		return nil, errors.New("manifest worktreeBinding.verifiedPorcelain must be true")
	}
	if m.WorktreeBinding.DirtyPolicy != "require_clean" && m.WorktreeBinding.DirtyPolicy != "allow_dirty_recorded" {
		return nil, fmt.Errorf("invalid worktree dirtyPolicy %q", m.WorktreeBinding.DirtyPolicy)
	}
	if m.WorktreeBinding.DirtyPolicy == "require_clean" && !m.WorktreeBinding.IsClean {
		return nil, errors.New("manifest worktreeBinding dirtyPolicy is require_clean but isClean is false")
	}
	if m.WorktreeBinding.IsClean && m.WorktreeBinding.PorcelainSha256 != "" {
		return nil, fmt.Errorf("manifest worktree isClean is true but porcelainSha256 is non-empty: %q", m.WorktreeBinding.PorcelainSha256)
	}
	if !m.WorktreeBinding.IsClean {
		if m.WorktreeBinding.DirtyPolicy != "allow_dirty_recorded" {
			return nil, fmt.Errorf("manifest worktree is not clean but dirtyPolicy is %q", m.WorktreeBinding.DirtyPolicy)
		}
		if !is64Hex(m.WorktreeBinding.PorcelainSha256) {
			return nil, fmt.Errorf("dirty worktree missing valid 64-hex porcelainSha256: %q", m.WorktreeBinding.PorcelainSha256)
		}
	}

	// 7. Consistency invariants across branch and HEAD
	if m.Target.Branch != m.SessionBinding.SessionBranch || m.Target.Branch != m.WorktreeBinding.WorktreeBranch {
		return nil, errors.New("manifest branch binding inconsistency between target, session, and worktree")
	}
	if m.Target.CurrentHead != m.WorktreeBinding.Head {
		return nil, errors.New("manifest currentHead does not match worktree head")
	}
	if m.Target.BaselineSha != m.Target.CurrentHead {
		return nil, errors.New("manifest baselineSha does not match target currentHead")
	}

	// 8. AttachedSessionProfile invariants
	if strings.TrimSpace(m.AttachedSessionProfile.Kind) == "" ||
		strings.TrimSpace(m.AttachedSessionProfile.Harness) == "" ||
		strings.TrimSpace(m.AttachedSessionProfile.Model) == "" ||
		strings.TrimSpace(m.AttachedSessionProfile.ReasoningEffort) == "" {
		return nil, errors.New("manifest missing attachedSessionProfile fields")
	}
	if m.AttachedSessionProfile.Status != "VERIFIED" {
		return nil, fmt.Errorf("manifest attachedSessionProfile.status %q must be VERIFIED", m.AttachedSessionProfile.Status)
	}

	// 9. Endpoint invariants
	if err := ValidateLoopbackURL(m.Endpoints.AOURL); err != nil {
		return nil, fmt.Errorf("manifest endpoints.aoUrl invalid: %w", err)
	}
	if strings.TrimSpace(m.Endpoints.AOIdentity) == "" {
		return nil, errors.New("manifest endpoints.aoIdentity cannot be empty")
	}
	switch m.Endpoints.AODiscoverySource {
	case "run_file":
		if m.Endpoints.AOPid <= 0 || m.Endpoints.AOPidStatus != "VERIFIED" {
			return nil, fmt.Errorf("run_file discovery must have pid > 0 and aoPidStatus VERIFIED; got pid=%d, status=%s",
				m.Endpoints.AOPid, m.Endpoints.AOPidStatus)
		}
	case "explicit_url", "environment_url":
		if m.Endpoints.AOPid != 0 || m.Endpoints.AOPidStatus != "NOT_OBSERVED" {
			return nil, fmt.Errorf("%s discovery must have pid=0 and aoPidStatus NOT_OBSERVED; got pid=%d, status=%s",
				m.Endpoints.AODiscoverySource, m.Endpoints.AOPid, m.Endpoints.AOPidStatus)
		}
	default:
		return nil, fmt.Errorf("unknown manifest aoDiscoverySource %q", m.Endpoints.AODiscoverySource)
	}

	if err := ValidateLoopbackURL(m.Endpoints.GatewayProbe.URL); err != nil {
		return nil, fmt.Errorf("manifest endpoints.gatewayProbe.url invalid: %w", err)
	}
	if m.Endpoints.GatewayProbe.CatalogPath != "/v1/models" || m.Endpoints.GatewayProbe.CatalogStatus != "VERIFIED" {
		return nil, errors.New("invalid gatewayProbe catalogPath or catalogStatus in manifest")
	}
	if m.Endpoints.GatewayProbe.ProviderCallPerformed {
		return nil, errors.New("manifest gatewayProbe.providerCallPerformed must be false")
	}
	if m.Endpoints.GatewayProbe.CredentialEligibility != "NOT_OBSERVED" {
		return nil, errors.New("manifest gatewayProbe.credentialEligibility must be NOT_OBSERVED")
	}
	if len(m.Endpoints.GatewayProbe.ObservedModels) == 0 {
		return nil, errors.New("manifest gatewayProbe.observedModels is empty")
	}

	// attachedSessionProfile.model must exist in gatewayProbe.observedModels
	modelInCatalog := false
	for _, mod := range m.Endpoints.GatewayProbe.ObservedModels {
		if mod == m.AttachedSessionProfile.Model {
			modelInCatalog = true
			break
		}
	}
	if !modelInCatalog {
		return nil, fmt.Errorf("manifest attachedSessionProfile.model %q not found in endpoints.gatewayProbe.observedModels", m.AttachedSessionProfile.Model)
	}

	// 10. Source provenance invariants
	if !is40Hex(m.SourceProvenance.CLIProxyAPISha) {
		return nil, fmt.Errorf("manifest sourceProvenance.cliProxyApiSha %q is not a valid 40-hex Git SHA", m.SourceProvenance.CLIProxyAPISha)
	}
	if !m.SourceProvenance.SourceAuditOnly {
		return nil, errors.New("manifest sourceProvenance.sourceAuditOnly must be true")
	}

	// 11. Lease invariants
	if m.Lease.RunOwner != m.RunID || m.Lease.OwnerID != m.RunID || m.Lease.TaskID != "__run__" {
		return nil, errors.New("manifest lease identifiers mismatch runId or task '__run__'")
	}
	if m.Lease.ObservedState == recovery.LeaseStateLocked {
		return nil, errors.New("manifest lease observedState cannot be LOCKED in an attached manifest")
	}
	if m.Lease.ObservedState == recovery.LeaseStateFree && m.Lease.ObservedPID != 0 {
		return nil, fmt.Errorf("manifest lease is FREE but observedPid is %d (must be 0)", m.Lease.ObservedPID)
	}
	if m.Lease.ObservedState == recovery.LeaseStateOwned && m.Lease.ObservedPID <= 0 {
		return nil, fmt.Errorf("manifest lease is OWNED but observedPid is %d (must be > 0)", m.Lease.ObservedPID)
	}
	if m.Lease.ObservedState != recovery.LeaseStateFree && m.Lease.ObservedState != recovery.LeaseStateOwned {
		return nil, fmt.Errorf("unknown lease observedState %q in manifest", m.Lease.ObservedState)
	}

	return &m, nil
}

// IsExactBindingMatch verifies whether an existing manifest matches the expected run and bindings.
// Separates immutable bindings from live observations.
//
// Compares immutable bindings:
// - runId, runSpecSha256;
// - target root, execution workspace, baseline/current HEAD, branch, repository identity;
// - session ID, project ID, branch;
// - worktree path, branch, HEAD, dirty policy, isClean, porcelain hash, verifiedPorcelain;
// - attached profile;
// - lease workspaceRoot, runOwner, taskId, ownerId;
// - AO URL + AO identity;
// - Gateway URL;
// - source provenance.
//
// Ignores live observations:
// - generatedAt;
// - lease.observedState, lease.observedPid;
// - AO PID, PID status, discovery source;
// - gateway observedModels;
// - catalog observations that can change without altering the binding.
func IsExactBindingMatch(existing, expected *RunManifest) bool {
	if existing == nil || expected == nil {
		return false
	}
	if existing.SchemaVersion != expected.SchemaVersion {
		return false
	}
	if existing.RunID != expected.RunID {
		return false
	}
	if existing.RunSpecSha256 != expected.RunSpecSha256 {
		return false
	}
	if !arePathsSameOrEquivalent(existing.Target.TargetRoot, expected.Target.TargetRoot) {
		return false
	}
	if !arePathsSameOrEquivalent(existing.Target.ExecutionWorkspace, expected.Target.ExecutionWorkspace) {
		return false
	}
	if existing.Target.BaselineSha != expected.Target.BaselineSha {
		return false
	}
	if existing.Target.CurrentHead != expected.Target.CurrentHead {
		return false
	}
	if existing.Target.Branch != expected.Target.Branch {
		return false
	}
	if !arePathsSameOrEquivalent(existing.Target.RepositoryIdentity, expected.Target.RepositoryIdentity) {
		return false
	}
	if existing.SessionBinding.SessionID != expected.SessionBinding.SessionID {
		return false
	}
	if existing.SessionBinding.ProjectID != expected.SessionBinding.ProjectID {
		return false
	}
	if existing.SessionBinding.SessionBranch != expected.SessionBinding.SessionBranch {
		return false
	}
	if !arePathsSameOrEquivalent(existing.WorktreeBinding.CanonicalPath, expected.WorktreeBinding.CanonicalPath) {
		return false
	}
	if existing.WorktreeBinding.WorktreeBranch != expected.WorktreeBinding.WorktreeBranch {
		return false
	}
	if existing.WorktreeBinding.Head != expected.WorktreeBinding.Head {
		return false
	}
	if existing.WorktreeBinding.DirtyPolicy != expected.WorktreeBinding.DirtyPolicy {
		return false
	}
	if existing.WorktreeBinding.IsClean != expected.WorktreeBinding.IsClean {
		return false
	}
	if existing.WorktreeBinding.PorcelainSha256 != expected.WorktreeBinding.PorcelainSha256 {
		return false
	}
	if existing.WorktreeBinding.VerifiedPorcelain != expected.WorktreeBinding.VerifiedPorcelain {
		return false
	}
	if existing.AttachedSessionProfile != expected.AttachedSessionProfile {
		return false
	}
	if !arePathsSameOrEquivalent(existing.Lease.WorkspaceRoot, expected.Lease.WorkspaceRoot) {
		return false
	}
	if existing.Lease.RunOwner != expected.Lease.RunOwner ||
		existing.Lease.TaskID != expected.Lease.TaskID ||
		existing.Lease.OwnerID != expected.Lease.OwnerID {
		return false
	}
	if existing.Endpoints.AOURL != expected.Endpoints.AOURL ||
		existing.Endpoints.AOIdentity != expected.Endpoints.AOIdentity {
		return false
	}
	if existing.Endpoints.GatewayProbe.URL != expected.Endpoints.GatewayProbe.URL {
		return false
	}
	if existing.SourceProvenance != expected.SourceProvenance {
		return false
	}
	return true
}

// WriteManifestAtomic writes a RunManifest atomically to disk.
// If the file already exists:
// - If it matches exact bindings, it returns nil (idempotent success) without rewriting.
// - If corrupt or mismatched, it returns an error (fail-closed, does not overwrite).
func WriteManifestAtomic(targetPath string, manifest *RunManifest) error {
	if strings.TrimSpace(targetPath) == "" {
		return errors.New("manifest target path is required")
	}
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("canonicalize manifest target path: %w", err)
	}

	// 1. Check if target already exists
	if _, err := os.Stat(absPath); err == nil {
		existing, readErr := ReadManifest(absPath)
		if readErr != nil {
			return fmt.Errorf("existing manifest at %s is invalid or corrupt: %w", absPath, readErr)
		}
		if IsExactBindingMatch(existing, manifest) {
			// Idempotent success without rewriting
			return nil
		}
		return fmt.Errorf("manifest already exists at %s with mismatched bindings or runId", absPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat existing manifest at %s: %w", absPath, err)
	}

	// 2. Prepare JSON data
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest json: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}

	// 3. Create temp file in the same directory
	var randBytes [8]byte
	_, _ = rand.Read(randBytes[:])
	tmpPath := filepath.Join(dir, fmt.Sprintf(".manifest-%s.tmp", hex.EncodeToString(randBytes[:])))

	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create temp manifest file: %w", err)
	}

	writeSuccess := false
	defer func() {
		if !writeSuccess {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write temp manifest: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync temp manifest: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp manifest: %w", err)
	}

	// 4. Atomic rename
	if err := os.Rename(tmpPath, absPath); err != nil {
		// Handle potential race where target appeared during rename
		if existing, readErr := ReadManifest(absPath); readErr == nil && IsExactBindingMatch(existing, manifest) {
			_ = os.Remove(tmpPath)
			writeSuccess = true
			return nil
		}
		return fmt.Errorf("atomic rename manifest from %s to %s: %w", tmpPath, absPath, err)
	}

	writeSuccess = true
	return nil
}
