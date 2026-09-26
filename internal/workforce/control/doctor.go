package control

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DoctorOptions holds CLI parameters for the doctor command.
type DoctorOptions struct {
	SpecPath   string
	AOURL      string
	GatewayURL string
	JSON       bool
}

// ParseDoctorFlags parses CLI arguments for coworkers doctor.
func ParseDoctorFlags(args []string, stdout, stderr io.Writer) (*DoctorOptions, int, error) {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var specPath, aoURL, gwURL string
	var jsonOutput bool
	var manifestForbidden string

	fs.StringVar(&specPath, "spec", "", "Path to RunSpec JSON file (required)")
	fs.StringVar(&aoURL, "ao-url", "", "Explicit AO daemon loopback URL")
	fs.StringVar(&gwURL, "gateway-url", "", "Explicit gateway loopback URL")
	fs.BoolVar(&jsonOutput, "json", false, "Output doctor report in JSON format")
	fs.StringVar(&manifestForbidden, "manifest", "", "Forbidden in doctor command")

	if err := fs.Parse(args); err != nil {
		return nil, ExitCodeUsageOrUnsupported, err
	}

	// doctor does NOT accept --manifest
	if manifestForbidden != "" {
		return nil, ExitCodeValidation, errors.New("doctor command does not accept --manifest flag")
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "--manifest") || strings.HasPrefix(arg, "-manifest") {
			return nil, ExitCodeValidation, errors.New("doctor command does not accept --manifest flag")
		}
	}

	if strings.TrimSpace(specPath) == "" {
		return nil, ExitCodeValidation, errors.New("--spec <run-spec.json> is required")
	}

	return &DoctorOptions{
		SpecPath:   specPath,
		AOURL:      aoURL,
		GatewayURL: gwURL,
		JSON:       jsonOutput,
	}, ExitCodeSuccess, nil
}

// RunDoctor executes the doctor health checks according to the Phase 8A specification.
func RunDoctor(ctx context.Context, opts *DoctorOptions, stdout, stderr io.Writer) (*DoctorReport, int) {
	report := &DoctorReport{
		Status:        "FAIL",
		ModelPresence: make(map[string]bool),
	}

	// 1. Read and validate RunSpec
	rawSpec, err := os.ReadFile(opts.SpecPath)
	if err != nil {
		fmt.Fprintf(stderr, "doctor: read spec file %s: %v\n", opts.SpecPath, err)
		report.Errors = append(report.Errors, fmt.Sprintf("read spec file: %v", err))
		return report, ExitCodeValidation
	}
	spec, rawSha, err := ParseAndValidateRunSpec(rawSpec)
	if err != nil {
		fmt.Fprintf(stderr, "doctor: validate spec: %v\n", err)
		report.Errors = append(report.Errors, fmt.Sprintf("validate spec: %v", err))
		return report, ExitCodeValidation
	}
	report.RunID = spec.RunID
	report.RunSpecSha256 = rawSha

	// 2. Discover and probe AO Daemon
	aoClient := &http.Client{Timeout: 5 * time.Second}
	aoRes, dErr := ResolveAOEndpoint(ctx, opts.AOURL, aoClient)
	if dErr != nil {
		fmt.Fprintf(stderr, "doctor: resolve AO endpoint: %v\n", dErr.Err)
		report.Errors = append(report.Errors, fmt.Sprintf("resolve AO endpoint: %v", dErr.Err))
		return report, int(dErr.Kind)
	}
	report.AOEndpoint = *aoRes

	// 3. Discover and probe Gateway
	gwClient := &http.Client{Timeout: 5 * time.Second}
	gwRes, gwErr := ResolveGatewayEndpoint(ctx, opts.GatewayURL, gwClient)
	if gwErr != nil {
		fmt.Fprintf(stderr, "doctor: resolve gateway endpoint: %v\n", gwErr.Err)
		report.Errors = append(report.Errors, fmt.Sprintf("resolve gateway endpoint: %v", gwErr.Err))
		return report, int(gwErr.Kind)
	}
	report.GatewayEndpoint = *gwRes

	// 4. Validate TargetRoot is clean git repo
	targetRootClean, err := checkRepoClean(ctx, spec.Target.TargetRoot)
	if err != nil {
		fmt.Fprintf(stderr, "doctor: check product root clean: %v\n", err)
		report.Errors = append(report.Errors, fmt.Sprintf("check product root: %v", err))
		return report, ExitCodeValidation
	}
	if !targetRootClean {
		fmt.Fprintf(stderr, "doctor: product root %s is dirty (must be clean)\n", spec.Target.TargetRoot)
		report.Errors = append(report.Errors, "product root is dirty")
		return report, ExitCodeValidation
	}
	report.TargetRootClean = true

	// 5. Validate ExecutionWorkspace dirty policy
	isClean, porcelainSha, err := checkWorktreeDirtyPolicy(ctx, spec.Target.ExecutionWorkspace, spec.RunID)
	if err != nil {
		fmt.Fprintf(stderr, "doctor: check execution workspace: %v\n", err)
		report.Errors = append(report.Errors, fmt.Sprintf("check execution workspace: %v", err))
		return report, ExitCodeValidation
	}
	report.WorkspaceCheck = WorkspaceCheck{
		Policy:          spec.WorkspacePolicy.ExecutionWorkspaceDirtyPolicy,
		IsClean:         isClean,
		PorcelainSha256: porcelainSha,
	}
	if spec.WorkspacePolicy.ExecutionWorkspaceDirtyPolicy == "require_clean" && !isClean {
		fmt.Fprintf(stderr, "doctor: execution workspace is dirty but dirtyPolicy is require_clean\n")
		report.Errors = append(report.Errors, "execution workspace dirty with require_clean policy")
		return report, ExitCodeValidation
	}

	// 6. Validate Baseline commit exists
	if err := checkGitCommitExists(ctx, spec.Target.TargetRoot, spec.Target.BaselineSha); err != nil {
		fmt.Fprintf(stderr, "doctor: baseline commit %s not found in target repo: %v\n", spec.Target.BaselineSha, err)
		report.Errors = append(report.Errors, fmt.Sprintf("baseline commit check: %v", err))
		return report, ExitCodeValidation
	}
	report.BaselineFound = true

	// 7. Validate RequiredTools via exec.LookPath
	allToolsPass := true
	for _, tool := range spec.RequiredTools {
		res := ToolCheckResult{
			Name:     tool.Name,
			Required: tool.Required,
		}
		p, err := exec.LookPath(tool.Name)
		if err == nil {
			res.Found = true
			res.Path = p
		} else {
			res.Found = false
			if tool.Required {
				allToolsPass = false
				report.Errors = append(report.Errors, fmt.Sprintf("required tool %q not found in PATH", tool.Name))
			}
		}
		report.ToolsCheck = append(report.ToolsCheck, res)
	}
	if !allToolsPass {
		return report, ExitCodeValidation
	}

	// 8. Check model catalog presence for declared profiles (every model must be present)
	observedSet := make(map[string]struct{})
	for _, m := range gwRes.ObservedModels {
		observedSet[m] = struct{}{}
	}
	var missingModels []string
	for profName, prof := range spec.Profiles {
		_, exists := observedSet[prof.Model]
		report.ModelPresence[prof.Model] = exists
		if !exists {
			missingModels = append(missingModels, fmt.Sprintf("%s (in profile %s)", prof.Model, profName))
		}
	}
	if len(missingModels) > 0 {
		fmt.Fprintf(stderr, "doctor: required model(s) not found in gateway catalog: %s\n", strings.Join(missingModels, ", "))
		report.Errors = append(report.Errors, fmt.Sprintf("missing models in catalog: %s", strings.Join(missingModels, ", ")))
		return report, ExitCodeValidation
	}

	report.Status = "PASS"
	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		fmt.Fprintf(stdout, "PASS: Run %s preflight checks succeeded.\n", report.RunID)
		fmt.Fprintf(stdout, "AO Endpoint: %s (Identity: %s, PID: %d, Status: %s)\n",
			report.AOEndpoint.URL, report.AOEndpoint.Identity, report.AOEndpoint.PID, report.AOEndpoint.PIDStatus)
		fmt.Fprintf(stdout, "Gateway: %s (Catalog: %s, CredentialEligibility: %s)\n",
			report.GatewayEndpoint.URL, report.GatewayEndpoint.CatalogStatus, report.GatewayEndpoint.CredentialEligibility)
		fmt.Fprintf(stdout, "Target Root Clean: true, Workspace Clean: %v\n", report.WorkspaceCheck.IsClean)
	}

	return report, ExitCodeSuccess
}

func checkRepoClean(ctx context.Context, repoPath string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "status", "--porcelain=v1", "-z")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return len(out) == 0, nil
}

func filterExactLeaseArtifact(raw []byte, expectedLeaseRelPath string) []byte {
	if len(raw) == 0 {
		return raw
	}
	tokens := bytes.Split(raw, []byte{0})
	var kept [][]byte
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if len(token) == 0 {
			continue
		}
		// Handle rename/copy records fail-closed: XY <orig>\0<dest>\0
		if len(token) >= 2 && (token[0] == 'R' || token[0] == 'C' || token[1] == 'R' || token[1] == 'C') {
			kept = append(kept, token)
			if i+1 < len(tokens) && len(tokens[i+1]) > 0 {
				i++
				kept = append(kept, tokens[i])
			}
			continue
		}
		// Only exact untracked record `?? ` qualifies for the lease exception.
		// All other statuses (A ,  M, M ,  D, D , type change, unmerged, malformed) are kept.
		if len(token) < 3 || token[0] != '?' || token[1] != '?' || token[2] != ' ' {
			kept = append(kept, token)
			continue
		}
		pathPart := token[3:]
		pathStr := filepath.ToSlash(string(pathPart))
		if pathStr == expectedLeaseRelPath {
			// Exactly matches untracked expected same-run lease artifact: filter out
			continue
		}
		kept = append(kept, token)
	}
	if len(kept) == 0 {
		return nil
	}
	var res []byte
	for _, k := range kept {
		res = append(res, k...)
		res = append(res, 0)
	}
	return res
}

func checkWorktreeDirtyPolicy(ctx context.Context, worktreePath, runID string) (bool, string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "status", "--porcelain=v1", "-uall", "-z")
	out, err := cmd.Output()
	if err != nil {
		return false, "", fmt.Errorf("git status on worktree: %w", err)
	}
	if len(out) == 0 {
		return true, "", nil
	}

	key := sha256.Sum256([]byte(runID + "\n__run__"))
	expectedLeaseRelPath := fmt.Sprintf(".agents-coworkers/recovery-leases/%x.lease", key)

	filtered := filterExactLeaseArtifact(out, expectedLeaseRelPath)
	if len(filtered) == 0 {
		return true, "", nil
	}
	h := sha256.Sum256(filtered)
	return false, hex.EncodeToString(h[:]), nil
}

func checkGitCommitExists(ctx context.Context, repoPath, commitSha string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "cat-file", "-e", commitSha+"^{commit}")
	return cmd.Run()
}
