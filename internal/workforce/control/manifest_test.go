package control

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trungqwe/agents-coworkers/internal/recovery"
)

const sample64Hex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
const sample40Hex = "0123456789abcdef0123456789abcdef01234567"

func sampleManifest(runID, targetDir string) *RunManifest {
	absDir, _ := filepath.Abs(targetDir)
	return &RunManifest{
		SchemaVersion: RunManifestSchemaVersion,
		RunID:         runID,
		RunSpecSha256: sample64Hex,
		GeneratedAt:   "2026-10-01T00:00:00Z",
		GeneratedBy:   "coworkers attach",
		Target: TargetManifest{
			TargetRoot:         absDir,
			ExecutionWorkspace: absDir,
			BaselineSha:        "commit123",
			CurrentHead:        "commit123",
			Branch:             "feature/x",
			RepositoryIdentity: absDir,
		},
		Endpoints: EndpointsManifest{
			AOURL:             "http://127.0.0.1:3000",
			AOIdentity:        "ao-host-01",
			AODiscoverySource: "run_file",
			AOPid:             4567,
			AOPidStatus:       "VERIFIED",
			GatewayProbe: GatewayProbeManifest{
				URL:                   "http://127.0.0.1:8317",
				CatalogPath:           "/v1/models",
				CatalogStatus:         "VERIFIED",
				ObservedModels:        []string{"gpt-5"},
				ProviderCallPerformed: false,
				CredentialEligibility: "NOT_OBSERVED",
			},
		},
		SessionBinding: SessionBindingManifest{
			SessionID:     "sess-01",
			ProjectID:     "proj-01",
			SessionBranch: "feature/x",
		},
		WorktreeBinding: WorktreeBindingManifest{
			CanonicalPath:     absDir,
			WorktreeBranch:    "feature/x",
			Head:              "commit123",
			IsClean:           true,
			DirtyPolicy:       "require_clean",
			PorcelainSha256:   "",
			VerifiedPorcelain: true,
		},
		AttachedSessionProfile: AttachedSessionProfile{
			Kind:            "orchestrator",
			Harness:         "codex",
			Model:           "gpt-5",
			ReasoningEffort: "low",
			Status:          "VERIFIED",
		},
		Lease: LeaseManifest{
			WorkspaceRoot: absDir,
			RunOwner:      runID,
			TaskID:        "__run__",
			OwnerID:       runID,
			ObservedState: "FREE",
			ObservedPID:   0,
		},
		SourceProvenance: SourceProvenance{
			CLIProxyAPISha:  sample40Hex,
			SourceAuditOnly: true,
		},
	}
}

func TestWriteManifestAtomic_CreateAndIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "run-manifest.json")

	m := sampleManifest("run-001", tmpDir)

	// 1. Initial write
	if err := WriteManifestAtomic(manifestPath, m); err != nil {
		t.Fatalf("WriteManifestAtomic failed: %v", err)
	}

	// 2. Read back
	readM, err := ReadManifest(manifestPath)
	if err != nil {
		t.Fatalf("ReadManifest failed: %v", err)
	}
	if readM.RunID != "run-001" {
		t.Errorf("expected runId run-001, got %s", readM.RunID)
	}

	// 3. Idempotent second write
	if err := WriteManifestAtomic(manifestPath, m); err != nil {
		t.Fatalf("idempotent WriteManifestAtomic failed: %v", err)
	}

	// 4. Mismatch write fails
	otherM := sampleManifest("run-002", tmpDir)
	if err := WriteManifestAtomic(manifestPath, otherM); err == nil {
		t.Fatal("expected error on mismatched manifest rewrite, got nil")
	}

	// 5. Corrupt existing manifest fails
	if err := os.WriteFile(manifestPath, []byte("corrupt-json"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := WriteManifestAtomic(manifestPath, m); err == nil {
		t.Fatal("expected error when existing manifest is corrupt, got nil")
	}
}

func TestReadManifest_TrailingTokensAndUnknownFields(t *testing.T) {
	tmpDir := t.TempDir()
	m := sampleManifest("run-001", tmpDir)
	manifestPath := filepath.Join(tmpDir, "manifest.json")
	if err := WriteManifestAtomic(manifestPath, m); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	content, _ := os.ReadFile(manifestPath)

	// 1. Trailing token
	trailingPath := filepath.Join(tmpDir, "trailing.json")
	_ = os.WriteFile(trailingPath, append(content, []byte(` {"extra": 1}`)...), 0o644)
	if _, err := ReadManifest(trailingPath); err == nil {
		t.Fatal("expected error on trailing tokens, got nil")
	}

	// 2. Unknown field
	unknownPath := filepath.Join(tmpDir, "unknown.json")
	modContent := strings.Replace(string(content), `"runId": "run-001"`, `"runId": "run-001", "unknownField": "bad"`, 1)
	_ = os.WriteFile(unknownPath, []byte(modContent), 0o644)
	if _, err := ReadManifest(unknownPath); err == nil {
		t.Fatal("expected error on unknown field in manifest, got nil")
	}
}

func TestReadManifest_InvariantsStrictness(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. worktree canonicalPath differs from executionWorkspace
	m := sampleManifest("run-inv-01", tmpDir)
	m.WorktreeBinding.CanonicalPath = filepath.Clean("/other/path")
	p := filepath.Join(tmpDir, "inv1.json")
	data, _ := json.Marshal(m)
	_ = os.WriteFile(p, data, 0o644)
	if _, err := ReadManifest(p); err == nil {
		t.Fatal("expected error when worktree canonicalPath differs from executionWorkspace, got nil")
	}

	// 2. verifiedPorcelain = false
	m = sampleManifest("run-inv-02", tmpDir)
	m.WorktreeBinding.VerifiedPorcelain = false
	p = filepath.Join(tmpDir, "inv2.json")
	data, _ = json.Marshal(m)
	_ = os.WriteFile(p, data, 0o644)
	if _, err := ReadManifest(p); err == nil {
		t.Fatal("expected error when verifiedPorcelain is false, got nil")
	}

	// 3. require_clean but dirty (isClean = false)
	m = sampleManifest("run-inv-03", tmpDir)
	m.WorktreeBinding.DirtyPolicy = "require_clean"
	m.WorktreeBinding.IsClean = false
	m.WorktreeBinding.PorcelainSha256 = sample64Hex
	p = filepath.Join(tmpDir, "inv3.json")
	data, _ = json.Marshal(m)
	_ = os.WriteFile(p, data, 0o644)
	if _, err := ReadManifest(p); err == nil {
		t.Fatal("expected error when require_clean is dirty, got nil")
	}

	// 4. clean but has porcelain hash
	m = sampleManifest("run-inv-04", tmpDir)
	m.WorktreeBinding.IsClean = true
	m.WorktreeBinding.PorcelainSha256 = sample64Hex
	p = filepath.Join(tmpDir, "inv4.json")
	data, _ = json.Marshal(m)
	_ = os.WriteFile(p, data, 0o644)
	if _, err := ReadManifest(p); err == nil {
		t.Fatal("expected error when clean has porcelain hash, got nil")
	}

	// 5. attached model not in observedModels
	m = sampleManifest("run-inv-05", tmpDir)
	m.AttachedSessionProfile.Model = "nonexistent-model"
	p = filepath.Join(tmpDir, "inv5.json")
	data, _ = json.Marshal(m)
	_ = os.WriteFile(p, data, 0o644)
	if _, err := ReadManifest(p); err == nil {
		t.Fatal("expected error when attached model not in observedModels, got nil")
	}

	// 6. invalid generatedAt
	m = sampleManifest("run-inv-06", tmpDir)
	m.GeneratedAt = "not-a-timestamp"
	p = filepath.Join(tmpDir, "inv6.json")
	data, _ = json.Marshal(m)
	_ = os.WriteFile(p, data, 0o644)
	if _, err := ReadManifest(p); err == nil {
		t.Fatal("expected error on invalid generatedAt, got nil")
	}

	// 7. invalid source provenance
	m = sampleManifest("run-inv-07", tmpDir)
	m.SourceProvenance.CLIProxyAPISha = "not-40-hex"
	p = filepath.Join(tmpDir, "inv7.json")
	data, _ = json.Marshal(m)
	_ = os.WriteFile(p, data, 0o644)
	if _, err := ReadManifest(p); err == nil {
		t.Fatal("expected error on invalid cliProxyApiSha, got nil")
	}

	// 8. lease FREE but observedPid != 0
	m = sampleManifest("run-inv-08", tmpDir)
	m.Lease.ObservedState = "FREE"
	m.Lease.ObservedPID = 1234
	p = filepath.Join(tmpDir, "inv8.json")
	data, _ = json.Marshal(m)
	_ = os.WriteFile(p, data, 0o644)
	if _, err := ReadManifest(p); err == nil {
		t.Fatal("expected error when lease FREE has observedPid != 0, got nil")
	}

	// 9. lease OWNED but observedPid <= 0
	m = sampleManifest("run-inv-09", tmpDir)
	m.Lease.ObservedState = "OWNED"
	m.Lease.ObservedPID = 0
	p = filepath.Join(tmpDir, "inv9.json")
	data, _ = json.Marshal(m)
	_ = os.WriteFile(p, data, 0o644)
	if _, err := ReadManifest(p); err == nil {
		t.Fatal("expected error when lease OWNED has observedPid <= 0, got nil")
	}

	// 10. lease LOCKED in manifest
	m = sampleManifest("run-inv-10", tmpDir)
	m.Lease.ObservedState = "LOCKED"
	p = filepath.Join(tmpDir, "inv10.json")
	data, _ = json.Marshal(m)
	_ = os.WriteFile(p, data, 0o644)
	if _, err := ReadManifest(p); err == nil {
		t.Fatal("expected error when lease is LOCKED in manifest, got nil")
	}
}

func TestIsExactBindingMatch_IgnoresGeneratedAt(t *testing.T) {
	tmpDir := t.TempDir()
	m1 := sampleManifest("run-001", tmpDir)
	m2 := sampleManifest("run-001", tmpDir)
	m2.GeneratedAt = "2026-10-02T12:34:56Z" // different timestamp

	if !IsExactBindingMatch(m1, m2) {
		t.Error("expected IsExactBindingMatch to ignore generatedAt difference")
	}

	// But differences in profile or session fail
	m2.AttachedSessionProfile.Model = "gpt-other"
	if IsExactBindingMatch(m1, m2) {
		t.Error("expected IsExactBindingMatch to fail when model differs")
	}
}

func TestReadManifest_RejectsUncleanPathsWithDotDot(t *testing.T) {
	tmpDir := t.TempDir()
	cleanDir := filepath.Clean(tmpDir)
	uncleanDir := cleanDir + string(filepath.Separator) + "foo" + string(filepath.Separator) + ".."

	testCases := []struct {
		name   string
		mutate func(m *RunManifest)
	}{
		{
			name: "targetRoot with dotdot",
			mutate: func(m *RunManifest) {
				m.Target.TargetRoot = uncleanDir
			},
		},
		{
			name: "executionWorkspace with dotdot",
			mutate: func(m *RunManifest) {
				m.Target.ExecutionWorkspace = uncleanDir
				m.WorktreeBinding.CanonicalPath = uncleanDir
				m.Lease.WorkspaceRoot = uncleanDir
			},
		},
		{
			name: "repositoryIdentity with dotdot",
			mutate: func(m *RunManifest) {
				m.Target.RepositoryIdentity = uncleanDir
			},
		},
		{
			name: "worktreeBinding.canonicalPath with dotdot",
			mutate: func(m *RunManifest) {
				m.WorktreeBinding.CanonicalPath = uncleanDir
			},
		},
		{
			name: "lease.workspaceRoot with dotdot",
			mutate: func(m *RunManifest) {
				m.Lease.WorkspaceRoot = uncleanDir
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := sampleManifest("run-dotdot", cleanDir)
			tc.mutate(m)
			p := filepath.Join(tmpDir, "dotdot-"+tc.name+".json")
			data, _ := json.Marshal(m)
			_ = os.WriteFile(p, data, 0o644)
			if _, err := ReadManifest(p); err == nil {
				t.Fatalf("expected ReadManifest to reject unclean path with .., got nil for %s", tc.name)
			}
		})
	}
}

func TestIsExactBindingMatch_IgnoresLiveObservations(t *testing.T) {
	tmpDir := t.TempDir()
	m1 := sampleManifest("run-live-01", tmpDir)
	m2 := sampleManifest("run-live-01", tmpDir)

	// 1. Ignores generatedAt
	m2.GeneratedAt = "2026-10-02T12:34:56Z"
	if !IsExactBindingMatch(m1, m2) {
		t.Fatal("expected IsExactBindingMatch to ignore generatedAt")
	}

	// 2. Ignores live lease state and observedPid (e.g. FREE/0 vs OWNED/9999)
	m2.Lease.ObservedState = recovery.LeaseStateOwned
	m2.Lease.ObservedPID = 9999
	if !IsExactBindingMatch(m1, m2) {
		t.Fatal("expected IsExactBindingMatch to ignore live lease state/PID")
	}

	// 3. Ignores AO PID, PID status, and discovery source
	m2.Endpoints.AOPid = 8888
	m2.Endpoints.AOPidStatus = "NOT_OBSERVED"
	m2.Endpoints.AODiscoverySource = "explicit_url"
	if !IsExactBindingMatch(m1, m2) {
		t.Fatal("expected IsExactBindingMatch to ignore AO PID, status, and discovery source")
	}

	// 4. Ignores gateway observedModels changes if attached model is still present
	m2.Endpoints.GatewayProbe.ObservedModels = []string{"gpt-5", "claude-3-opus", "gemini-1.5-pro"}
	if !IsExactBindingMatch(m1, m2) {
		t.Fatal("expected IsExactBindingMatch to ignore gateway observedModels changes")
	}
}

func TestIsExactBindingMatch_FailsOnImmutableBindingDifference(t *testing.T) {
	tmpDir := t.TempDir()
	cleanDir := filepath.Clean(tmpDir)

	testCases := []struct {
		name   string
		mutate func(m *RunManifest)
	}{
		{"runId", func(m *RunManifest) { m.RunID = "other-run" }},
		{"runSpecSha256", func(m *RunManifest) { m.RunSpecSha256 = sample64Hex[:63] + "0" }},
		{"target.targetRoot", func(m *RunManifest) { m.Target.TargetRoot = filepath.Clean("/other/target/root") }},
		{"target.executionWorkspace", func(m *RunManifest) { m.Target.ExecutionWorkspace = filepath.Clean("/other/exec/ws") }},
		{"target.baselineSha", func(m *RunManifest) { m.Target.BaselineSha = "other-sha" }},
		{"target.currentHead", func(m *RunManifest) { m.Target.CurrentHead = "other-head" }},
		{"target.branch", func(m *RunManifest) { m.Target.Branch = "other-branch" }},
		{"target.repositoryIdentity", func(m *RunManifest) { m.Target.RepositoryIdentity = filepath.Clean("/other/repo/id") }},
		{"sessionBinding.sessionId", func(m *RunManifest) { m.SessionBinding.SessionID = "other-sess" }},
		{"sessionBinding.projectId", func(m *RunManifest) { m.SessionBinding.ProjectID = "other-proj" }},
		{"sessionBinding.sessionBranch", func(m *RunManifest) { m.SessionBinding.SessionBranch = "other-sess-branch" }},
		{"worktreeBinding.canonicalPath", func(m *RunManifest) { m.WorktreeBinding.CanonicalPath = filepath.Clean("/other/wt/path") }},
		{"worktreeBinding.worktreeBranch", func(m *RunManifest) { m.WorktreeBinding.WorktreeBranch = "other-wt-branch" }},
		{"worktreeBinding.head", func(m *RunManifest) { m.WorktreeBinding.Head = "other-wt-head" }},
		{"worktreeBinding.dirtyPolicy", func(m *RunManifest) { m.WorktreeBinding.DirtyPolicy = "allow_dirty_recorded" }},
		{"worktreeBinding.isClean", func(m *RunManifest) { m.WorktreeBinding.IsClean = !m.WorktreeBinding.IsClean }},
		{"worktreeBinding.porcelainSha256", func(m *RunManifest) { m.WorktreeBinding.PorcelainSha256 = sample64Hex }},
		{"worktreeBinding.verifiedPorcelain", func(m *RunManifest) { m.WorktreeBinding.VerifiedPorcelain = false }},
		{"attachedSessionProfile.model", func(m *RunManifest) { m.AttachedSessionProfile.Model = "other-model" }},
		{"attachedSessionProfile.reasoningEffort", func(m *RunManifest) { m.AttachedSessionProfile.ReasoningEffort = "medium" }},
		{"attachedSessionProfile.kind", func(m *RunManifest) { m.AttachedSessionProfile.Kind = "planner" }},
		{"attachedSessionProfile.harness", func(m *RunManifest) { m.AttachedSessionProfile.Harness = "other-harness" }},
		{"lease.workspaceRoot", func(m *RunManifest) { m.Lease.WorkspaceRoot = filepath.Clean("/other/lease/root") }},
		{"lease.runOwner", func(m *RunManifest) { m.Lease.RunOwner = "other-run-owner" }},
		{"lease.taskId", func(m *RunManifest) { m.Lease.TaskID = "other-task" }},
		{"lease.ownerId", func(m *RunManifest) { m.Lease.OwnerID = "other-owner" }},
		{"endpoints.aoUrl", func(m *RunManifest) { m.Endpoints.AOURL = "http://127.0.0.1:9999" }},
		{"endpoints.aoIdentity", func(m *RunManifest) { m.Endpoints.AOIdentity = "other-ao-id" }},
		{"endpoints.gatewayProbe.url", func(m *RunManifest) { m.Endpoints.GatewayProbe.URL = "http://127.0.0.1:9998" }},
		{"sourceProvenance.cliProxyApiSha", func(m *RunManifest) { m.SourceProvenance.CLIProxyAPISha = "0000000000000000000000000000000000000000" }},
		{"sourceProvenance.sourceAuditOnly", func(m *RunManifest) { m.SourceProvenance.SourceAuditOnly = false }},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m1 := sampleManifest("run-immutable-01", cleanDir)
			m2 := sampleManifest("run-immutable-01", cleanDir)
			tc.mutate(m2)
			if IsExactBindingMatch(m1, m2) {
				t.Fatalf("expected IsExactBindingMatch to return false when %s differs", tc.name)
			}
		})
	}
}
