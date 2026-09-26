package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/trungqwe/agents-coworkers/internal/recovery"
)

// AttachOptions holds CLI parameters for the attach command.
type AttachOptions struct {
	SpecPath     string
	SessionID    string
	Workspace    string
	ManifestPath string
	AOURL        string
	GatewayURL   string
	JSON         bool
}

type sessionResponseWrapper struct {
	Session struct {
		ID           string `json:"id"`
		ProjectID    string `json:"projectId"`
		Kind         string `json:"kind"`
		Harness      string `json:"harness"`
		Model        string `json:"model"`
		Branch       string `json:"branch"`
		IsTerminated bool   `json:"isTerminated"`
	} `json:"session"`
}

type conversationSettingsWrapper struct {
	Settings struct {
		Model           string `json:"model"`
		ReasoningEffort string `json:"reasoningEffort"`
	} `json:"settings"`
}

type parsedWorktree struct {
	Path     string
	HEAD     string
	Branch   string
	Detached bool
	Bare     bool
}

// ParseAttachFlags parses CLI arguments for coworkers attach.
func ParseAttachFlags(args []string, stdout, stderr io.Writer) (*AttachOptions, int, error) {
	fs := flag.NewFlagSet("attach", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var specPath, sessionID, workspace, manifestPath, aoURL, gwURL string
	var jsonOutput bool

	fs.StringVar(&specPath, "spec", "", "Path to RunSpec JSON file (required)")
	fs.StringVar(&sessionID, "session", "", "AO Session ID (required)")
	fs.StringVar(&workspace, "workspace", "", "Path to execution workspace worktree (required)")
	fs.StringVar(&manifestPath, "manifest", "", "Output path for RunManifest JSON (required)")
	fs.StringVar(&aoURL, "ao-url", "", "Explicit AO daemon loopback URL")
	fs.StringVar(&gwURL, "gateway-url", "", "Explicit gateway loopback URL")
	fs.BoolVar(&jsonOutput, "json", false, "Output attach result in JSON format")

	if err := fs.Parse(args); err != nil {
		return nil, ExitCodeUsageOrUnsupported, err
	}

	if strings.TrimSpace(specPath) == "" {
		return nil, ExitCodeValidation, errors.New("--spec <run-spec.json> is required")
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, ExitCodeValidation, errors.New("--session <session-id> is required")
	}
	if strings.TrimSpace(workspace) == "" {
		return nil, ExitCodeValidation, errors.New("--workspace <path> is required")
	}
	if strings.TrimSpace(manifestPath) == "" {
		return nil, ExitCodeValidation, errors.New("--manifest <output-path> is required")
	}

	return &AttachOptions{
		SpecPath:     specPath,
		SessionID:    sessionID,
		Workspace:    workspace,
		ManifestPath: manifestPath,
		AOURL:        aoURL,
		GatewayURL:   gwURL,
		JSON:         jsonOutput,
	}, ExitCodeSuccess, nil
}

// arePathsSameOrEquivalent checks if two filesystem paths point to the exact same directory/file.
// It prioritizes os.SameFile for existing paths, and handles Windows case-equivalence.
func arePathsSameOrEquivalent(p1, p2 string) bool {
	p1Clean := filepath.Clean(p1)
	p2Clean := filepath.Clean(p2)
	if p1Clean == p2Clean {
		return true
	}
	fi1, err1 := os.Stat(p1Clean)
	fi2, err2 := os.Stat(p2Clean)
	if err1 == nil && err2 == nil && os.SameFile(fi1, fi2) {
		return true
	}
	p1Abs, err1 := filepath.Abs(p1Clean)
	p2Abs, err2 := filepath.Abs(p2Clean)
	if err1 == nil && err2 == nil {
		if p1Abs == p2Abs {
			return true
		}
		if runtime.GOOS == "windows" && strings.EqualFold(p1Abs, p2Abs) {
			return true
		}
	}
	return false
}

func canonicalPhysicalPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	eval, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return eval, nil
	}
	return abs, nil
}

// RunAttach executes the attach command according to the Phase 8A specification.
func RunAttach(ctx context.Context, opts *AttachOptions, stdout, stderr io.Writer) (*RunManifest, int) {
	// 1. Read and validate RunSpec
	rawSpec, err := os.ReadFile(opts.SpecPath)
	if err != nil {
		fmt.Fprintf(stderr, "attach: read spec file %s: %v\n", opts.SpecPath, err)
		return nil, ExitCodeValidation
	}
	spec, rawSha, err := ParseAndValidateRunSpec(rawSpec)
	if err != nil {
		fmt.Fprintf(stderr, "attach: validate spec: %v\n", err)
		return nil, ExitCodeValidation
	}

	// Canonicalize workspace from flag and verify it matches RunSpec
	canonicalFlagWorkspace, err := canonicalPhysicalPath(opts.Workspace)
	if err != nil {
		fmt.Fprintf(stderr, "attach: canonicalize workspace %s: %v\n", opts.Workspace, err)
		return nil, ExitCodeValidation
	}
	if !arePathsSameOrEquivalent(canonicalFlagWorkspace, spec.Target.ExecutionWorkspace) {
		fmt.Fprintf(stderr, "attach: --workspace (%s) does not match spec executionWorkspace (%s)\n",
			canonicalFlagWorkspace, spec.Target.ExecutionWorkspace)
		return nil, ExitCodeValidation
	}

	// 2. Discover endpoints
	aoClient := &http.Client{Timeout: 5 * time.Second}
	aoRes, dErr := ResolveAOEndpoint(ctx, opts.AOURL, aoClient)
	if dErr != nil {
		fmt.Fprintf(stderr, "attach: resolve AO endpoint: %v\n", dErr.Err)
		return nil, int(dErr.Kind)
	}

	gwClient := &http.Client{Timeout: 5 * time.Second}
	gwRes, gwErr := ResolveGatewayEndpoint(ctx, opts.GatewayURL, gwClient)
	if gwErr != nil {
		fmt.Fprintf(stderr, "attach: resolve gateway endpoint: %v\n", gwErr.Err)
		return nil, int(gwErr.Kind)
	}

	// 3. Query AO Session
	sessionURL := strings.TrimRight(aoRes.URL, "/") + "/api/v1/sessions/" + url.PathEscape(opts.SessionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sessionURL, nil)
	if err != nil {
		fmt.Fprintf(stderr, "attach: create session request: %v\n", err)
		return nil, ExitCodeValidation
	}
	resp, err := aoClient.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "attach: query session: %v\n", err)
		return nil, ExitCodeEndpointUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stderr, "attach: AO session read returned HTTP %d\n", resp.StatusCode)
		return nil, ExitCodeValidation
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		fmt.Fprintf(stderr, "attach: read session body: %v\n", err)
		return nil, ExitCodeValidation
	}
	var sessWrap sessionResponseWrapper
	if err := json.Unmarshal(body, &sessWrap); err != nil {
		fmt.Fprintf(stderr, "attach: decode session JSON: %v\n", err)
		return nil, ExitCodeValidation
	}
	session := sessWrap.Session
	if strings.TrimSpace(session.ID) == "" || session.ID != opts.SessionID {
		fmt.Fprintf(stderr, "attach: session ID mismatch or empty: expected %s, got %s\n", opts.SessionID, session.ID)
		return nil, ExitCodeValidation
	}
	if strings.TrimSpace(session.ProjectID) == "" {
		fmt.Fprintf(stderr, "attach: session %s has empty projectId\n", session.ID)
		return nil, ExitCodeValidation
	}
	if strings.TrimSpace(session.Branch) == "" {
		fmt.Fprintf(stderr, "attach: session %s has empty branch\n", session.ID)
		return nil, ExitCodeValidation
	}
	if strings.TrimSpace(session.Kind) == "" {
		fmt.Fprintf(stderr, "attach: session %s has empty kind\n", session.ID)
		return nil, ExitCodeValidation
	}
	if strings.TrimSpace(session.Harness) == "" {
		fmt.Fprintf(stderr, "attach: session %s has empty harness\n", session.ID)
		return nil, ExitCodeValidation
	}
	if strings.TrimSpace(session.Model) == "" {
		fmt.Fprintf(stderr, "attach: session %s has empty model\n", session.ID)
		return nil, ExitCodeValidation
	}
	if session.IsTerminated {
		fmt.Fprintf(stderr, "attach: session %s is terminated\n", session.ID)
		return nil, ExitCodeValidation
	}

	// 3.1 Session.Model must appear in Gateway observedModels catalog
	modelInCatalog := false
	for _, m := range gwRes.ObservedModels {
		if m == session.Model {
			modelInCatalog = true
			break
		}
	}
	if !modelInCatalog {
		fmt.Fprintf(stderr, "attach: session model %q is not present in gateway observedModels catalog\n", session.Model)
		return nil, ExitCodeValidation
	}

	// 3.2 Match profile by session.Kind and model
	profile, ok := spec.Profiles[session.Kind]
	if !ok {
		fmt.Fprintf(stderr, "attach: no profile declared in RunSpec for session kind %q\n", session.Kind)
		return nil, ExitCodeValidation
	}
	if session.Model != profile.Model {
		fmt.Fprintf(stderr, "attach: session model %q does not match profile model %q\n", session.Model, profile.Model)
		return nil, ExitCodeValidation
	}

	// 3.3 Query reasoningEffort and model from conversation settings (MANDATORY, NO FALLBACK)
	convURL := sessionURL + "/conversation?limit=1"
	convReq, err := http.NewRequestWithContext(ctx, http.MethodGet, convURL, nil)
	if err != nil {
		fmt.Fprintf(stderr, "attach: create conversation request: %v\n", err)
		return nil, ExitCodeValidation
	}
	convResp, err := aoClient.Do(convReq)
	if err != nil {
		fmt.Fprintf(stderr, "attach: query conversation settings: %v\n", err)
		return nil, ExitCodeEndpointUnavailable
	}
	defer convResp.Body.Close()

	if convResp.StatusCode != http.StatusOK {
		fmt.Fprintf(stderr, "attach: query conversation settings returned HTTP %d\n", convResp.StatusCode)
		return nil, ExitCodeValidation
	}
	convBody, err := io.ReadAll(io.LimitReader(convResp.Body, 1<<18))
	if err != nil {
		fmt.Fprintf(stderr, "attach: read conversation settings body: %v\n", err)
		return nil, ExitCodeValidation
	}
	var convWrap conversationSettingsWrapper
	if err := json.Unmarshal(convBody, &convWrap); err != nil {
		fmt.Fprintf(stderr, "attach: decode conversation settings: %v\n", err)
		return nil, ExitCodeValidation
	}

	// Validate conversation model non-empty and equals session.Model and profile.Model
	convModel := strings.TrimSpace(convWrap.Settings.Model)
	if convModel == "" {
		fmt.Fprintf(stderr, "attach: conversation settings returned empty model\n")
		return nil, ExitCodeValidation
	}
	if convModel != session.Model {
		fmt.Fprintf(stderr, "attach: conversation model %q does not match session model %q\n", convModel, session.Model)
		return nil, ExitCodeValidation
	}
	if convModel != profile.Model {
		fmt.Fprintf(stderr, "attach: conversation model %q does not match profile model %q\n", convModel, profile.Model)
		return nil, ExitCodeValidation
	}

	// Validate reasoningEffort non-empty and equals profile.ReasoningEffort
	reasoningEffort := strings.TrimSpace(convWrap.Settings.ReasoningEffort)
	if reasoningEffort == "" {
		fmt.Fprintf(stderr, "attach: conversation settings returned empty reasoningEffort\n")
		return nil, ExitCodeValidation
	}
	if reasoningEffort != profile.ReasoningEffort {
		fmt.Fprintf(stderr, "attach: conversation reasoningEffort %q does not match profile reasoningEffort %q\n",
			reasoningEffort, profile.ReasoningEffort)
		return nil, ExitCodeValidation
	}

	// 4. Strict 9-Step Worktree Binding Algorithm
	// 4.1 Read session via AO API and extract project ID + branch -> session.ProjectID, session.Branch
	// 4.2 Canonicalize targetRoot and explicit execution workspace path
	canonicalTargetRoot, err := canonicalPhysicalPath(spec.Target.TargetRoot)
	if err != nil {
		fmt.Fprintf(stderr, "attach: canonicalize targetRoot %s: %v\n", spec.Target.TargetRoot, err)
		return nil, ExitCodeValidation
	}

	// 4.3 Run git worktree list --porcelain -z on target repo
	worktrees, err := listGitWorktrees(ctx, canonicalTargetRoot)
	if err != nil {
		fmt.Fprintf(stderr, "attach: list git worktrees in %s: %v\n", canonicalTargetRoot, err)
		return nil, ExitCodeValidation
	}

	// 4.4 Find exactly one worktree whose branch matches session branch
	var matchingBranchWT []parsedWorktree
	for _, wt := range worktrees {
		if !wt.Detached && !wt.Bare && wt.Branch == session.Branch {
			matchingBranchWT = append(matchingBranchWT, wt)
		}
	}
	if len(matchingBranchWT) != 1 {
		fmt.Fprintf(stderr, "attach: expected exactly 1 worktree matching session branch %q, found %d\n",
			session.Branch, len(matchingBranchWT))
		return nil, ExitCodeValidation
	}
	targetWT := matchingBranchWT[0]

	// 4.5 Path canonical of worktree must match execution workspace
	wtCanonical, err := canonicalPhysicalPath(targetWT.Path)
	if err != nil {
		fmt.Fprintf(stderr, "attach: canonicalize worktree path %s: %v\n", targetWT.Path, err)
		return nil, ExitCodeValidation
	}
	if !arePathsSameOrEquivalent(wtCanonical, canonicalFlagWorkspace) {
		fmt.Fprintf(stderr, "attach: worktree path (%s) does not match execution workspace (%s)\n",
			wtCanonical, canonicalFlagWorkspace)
		return nil, ExitCodeValidation
	}

	// 4.6 HEAD of worktree must match expected baselineSha
	if targetWT.HEAD != spec.Target.BaselineSha {
		fmt.Fprintf(stderr, "attach: worktree HEAD (%s) does not match expected baselineSha (%s)\n",
			targetWT.HEAD, spec.Target.BaselineSha)
		return nil, ExitCodeValidation
	}

	// 4.7 Repository common-dir and identity must match target repo
	repoID, err := getGitRepositoryIdentity(ctx, canonicalTargetRoot)
	if err != nil {
		fmt.Fprintf(stderr, "attach: get repo identity for targetRoot: %v\n", err)
		return nil, ExitCodeValidation
	}
	wtRepoID, err := getGitRepositoryIdentity(ctx, canonicalFlagWorkspace)
	if err != nil {
		fmt.Fprintf(stderr, "attach: get repo identity for worktree: %v\n", err)
		return nil, ExitCodeValidation
	}
	if !arePathsSameOrEquivalent(repoID, wtRepoID) {
		fmt.Fprintf(stderr, "attach: repository identity mismatch: targetRoot=%s, worktree=%s\n", repoID, wtRepoID)
		return nil, ExitCodeValidation
	}

	// Check expectedBranch matches session/worktree branch
	if spec.Target.ExpectedBranch != session.Branch {
		fmt.Fprintf(stderr, "attach: expectedBranch (%s) does not match session/worktree branch (%s)\n",
			spec.Target.ExpectedBranch, session.Branch)
		return nil, ExitCodeValidation
	}

	// 5. Check Workspace Dirty Policy
	targetClean, err := checkRepoClean(ctx, canonicalTargetRoot)
	if err != nil || !targetClean {
		fmt.Fprintf(stderr, "attach: product root checkout %s is dirty (must be clean)\n", canonicalTargetRoot)
		return nil, ExitCodeValidation
	}

	isClean, porcelainSha, err := checkWorktreeDirtyPolicy(ctx, canonicalFlagWorkspace, spec.RunID)
	if err != nil {
		fmt.Fprintf(stderr, "attach: check execution workspace status: %v\n", err)
		return nil, ExitCodeValidation
	}
	if spec.WorkspacePolicy.ExecutionWorkspaceDirtyPolicy == "require_clean" && !isClean {
		fmt.Fprintf(stderr, "attach: execution workspace is dirty but dirtyPolicy is require_clean\n")
		return nil, ExitCodeValidation
	}

	// 6. Inspect Lease Read-Only
	leaseInsp, err := recovery.InspectLease(canonicalFlagWorkspace, spec.RunID, "__run__", spec.RunID)
	if err != nil {
		fmt.Fprintf(stderr, "attach: lease inspect failed: %v\n", err)
		return nil, ExitCodeValidation
	}
	if leaseInsp.ObservedState == recovery.LeaseStateLocked {
		fmt.Fprintf(stderr, "attach: lease is LOCKED by another owner (%s)\n", leaseInsp.OwnerID)
		return nil, ExitCodeValidation
	}

	// 7. Construct Manifest
	now := time.Now().UTC().Format(time.RFC3339)
	manifest := &RunManifest{
		SchemaVersion: RunManifestSchemaVersion,
		RunID:         spec.RunID,
		RunSpecSha256: rawSha,
		GeneratedAt:   now,
		GeneratedBy:   "coworkers attach",
		Target: TargetManifest{
			TargetRoot:         canonicalTargetRoot,
			ExecutionWorkspace: canonicalFlagWorkspace,
			BaselineSha:        spec.Target.BaselineSha,
			CurrentHead:        targetWT.HEAD,
			Branch:             session.Branch,
			RepositoryIdentity: repoID,
		},
		Endpoints: EndpointsManifest{
			AOURL:             aoRes.URL,
			AOIdentity:        aoRes.Identity,
			AODiscoverySource: aoRes.DiscoverySource,
			AOPid:             aoRes.PID,
			AOPidStatus:       aoRes.PIDStatus,
			GatewayProbe: GatewayProbeManifest{
				URL:                   gwRes.URL,
				CatalogPath:           "/v1/models",
				CatalogStatus:         gwRes.CatalogStatus,
				ObservedModels:        gwRes.ObservedModels,
				ProviderCallPerformed: false,
				CredentialEligibility: "NOT_OBSERVED",
			},
		},
		SessionBinding: SessionBindingManifest{
			SessionID:     session.ID,
			ProjectID:     session.ProjectID,
			SessionBranch: session.Branch,
		},
		WorktreeBinding: WorktreeBindingManifest{
			CanonicalPath:     canonicalFlagWorkspace,
			WorktreeBranch:    session.Branch,
			Head:              targetWT.HEAD,
			IsClean:           isClean,
			DirtyPolicy:       spec.WorkspacePolicy.ExecutionWorkspaceDirtyPolicy,
			PorcelainSha256:   porcelainSha,
			VerifiedPorcelain: true,
		},
		AttachedSessionProfile: AttachedSessionProfile{
			Kind:            session.Kind,
			Harness:         session.Harness,
			Model:           session.Model,
			ReasoningEffort: reasoningEffort,
			Status:          "VERIFIED",
		},
		Lease: LeaseManifest{
			WorkspaceRoot: canonicalFlagWorkspace,
			RunOwner:      spec.RunID,
			TaskID:        "__run__",
			OwnerID:       spec.RunID,
			ObservedState: leaseInsp.ObservedState,
			ObservedPID:   leaseInsp.ObservedPID,
		},
		SourceProvenance: SourceProvenance{
			CLIProxyAPISha:  "2430354330af80b645f9ffb1a51e1e7c72c4cc8e",
			SourceAuditOnly: true,
		},
	}

	// Atomic write manifest
	if err := WriteManifestAtomic(opts.ManifestPath, manifest); err != nil {
		fmt.Fprintf(stderr, "attach: write manifest atomic: %v\n", err)
		return nil, ExitCodeValidation
	}

	// If manifest already existed on disk and matched idempotently, load persisted manifest
	outputManifest := manifest
	if existing, err := ReadManifest(opts.ManifestPath); err == nil {
		outputManifest = existing
	}

	// 8. Output
	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(outputManifest)
	} else {
		fmt.Fprintf(stdout, "PASS: Attached session %s to workspace %s.\n", session.ID, canonicalFlagWorkspace)
		fmt.Fprintf(stdout, "Manifest written to %s.\n", opts.ManifestPath)
	}

	return outputManifest, ExitCodeSuccess
}

func listGitWorktrees(ctx context.Context, repoPath string) ([]parsedWorktree, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "worktree", "list", "--porcelain", "-z")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	// Output format: records separated by \0\0 or items by \0
	rawItems := bytes.Split(out, []byte{0})
	var worktrees []parsedWorktree
	var current parsedWorktree

	for _, item := range rawItems {
		s := string(item)
		if s == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = parsedWorktree{}
			}
			continue
		}
		if strings.HasPrefix(s, "worktree ") {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = parsedWorktree{}
			}
			current.Path = strings.TrimPrefix(s, "worktree ")
		} else if strings.HasPrefix(s, "HEAD ") {
			current.HEAD = strings.TrimPrefix(s, "HEAD ")
		} else if strings.HasPrefix(s, "branch ") {
			ref := strings.TrimPrefix(s, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		} else if s == "detached" {
			current.Detached = true
		} else if s == "bare" {
			current.Bare = true
		}
	}
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}
	return worktrees, nil
}

func getGitRepositoryIdentity(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--git-common-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w", err)
	}
	rawPath := strings.TrimSpace(string(out))
	if !filepath.IsAbs(rawPath) {
		rawPath = filepath.Join(dir, rawPath)
	}
	return canonicalPhysicalPath(rawPath)
}
