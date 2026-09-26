package control

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/trungqwe/agents-coworkers/internal/recovery"
)

// StatusOptions holds parsed CLI flags for the status command.
type StatusOptions struct {
	ManifestPath string
	AOURL        string
	GatewayURL   string
	JSON         bool
}

// StatusReport holds the output data for the status command.
type StatusReport struct {
	Status          string                   `json:"status"` // "OK" or "ERROR"
	RunID           string                   `json:"runId"`
	RunSpecSha256   string                   `json:"runSpecSha256"`
	GeneratedAt     string                   `json:"generatedAt"`
	Target          TargetManifest           `json:"target"`
	Endpoints       EndpointsManifest        `json:"endpoints"`
	SessionBinding  SessionBindingManifest   `json:"sessionBinding"`
	WorktreeBinding WorktreeBindingManifest  `json:"worktreeBinding"`
	AttachedProfile AttachedSessionProfile   `json:"attachedProfile"`
	LiveLease       recovery.LeaseInspection `json:"liveLease"`
}

// ParseStatusFlags parses CLI arguments for coworkers status.
func ParseStatusFlags(args []string, stdout, stderr io.Writer) (*StatusOptions, int, error) {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var manifestPath, aoURL, gwURL string
	var jsonOutput bool

	fs.StringVar(&manifestPath, "manifest", "", "Path to RunManifest JSON file (required)")
	fs.StringVar(&aoURL, "ao-url", "", "Explicit AO daemon loopback URL")
	fs.StringVar(&gwURL, "gateway-url", "", "Explicit gateway loopback URL")
	fs.BoolVar(&jsonOutput, "json", false, "Output status report in JSON format")

	if err := fs.Parse(args); err != nil {
		return nil, ExitCodeUsageOrUnsupported, err
	}

	if strings.TrimSpace(manifestPath) == "" {
		return nil, ExitCodeValidation, errors.New("--manifest <path-to-run-manifest.json> is required")
	}

	return &StatusOptions{
		ManifestPath: manifestPath,
		AOURL:        aoURL,
		GatewayURL:   gwURL,
		JSON:         jsonOutput,
	}, ExitCodeSuccess, nil
}

// RunStatus executes the status inspection command according to the Phase 8A specification.
func RunStatus(ctx context.Context, opts *StatusOptions, stdout, stderr io.Writer) (*StatusReport, int) {
	// 1. Read RunManifest (strictly without reading RunSpec)
	manifest, err := ReadManifest(opts.ManifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "status: read manifest %s: %v\n", opts.ManifestPath, err)
		return nil, ExitCodeValidation
	}

	// 2. Validate AO endpoint override if provided
	if opts.AOURL != "" {
		if err := ValidateLoopbackURL(opts.AOURL); err != nil {
			fmt.Fprintf(stderr, "status: --ao-url %q invalid: %v\n", opts.AOURL, err)
			return nil, ExitCodeValidation
		}
		aoClient := &http.Client{Timeout: 5 * time.Second}
		hostID, dErr := ProbeAOIdentity(ctx, aoClient, opts.AOURL)
		if dErr != nil {
			fmt.Fprintf(stderr, "status: probe AO identity at %s: %v\n", opts.AOURL, dErr.Err)
			return nil, int(dErr.Kind)
		}
		if manifest.Endpoints.AOIdentity != "" && hostID != manifest.Endpoints.AOIdentity {
			fmt.Fprintf(stderr, "status: AO identity mismatch: manifest has %q, probed %q\n",
				manifest.Endpoints.AOIdentity, hostID)
			return nil, ExitCodeValidation
		}
	}

	// 3. Validate Gateway endpoint override if provided
	if opts.GatewayURL != "" {
		if err := ValidateLoopbackURL(opts.GatewayURL); err != nil {
			fmt.Fprintf(stderr, "status: --gateway-url %q invalid: %v\n", opts.GatewayURL, err)
			return nil, ExitCodeValidation
		}
		// Gateway override must match bound endpoint and probe /v1/models successfully
		cleanOverride := strings.TrimRight(opts.GatewayURL, "/")
		cleanManifestGW := strings.TrimRight(manifest.Endpoints.GatewayProbe.URL, "/")
		if cleanOverride != cleanManifestGW {
			fmt.Fprintf(stderr, "status: gateway override %q does not match bound manifest gateway %q\n",
				cleanOverride, cleanManifestGW)
			return nil, ExitCodeValidation
		}
		gwClient := &http.Client{Timeout: 5 * time.Second}
		gwRes, gwErr := ResolveGatewayEndpoint(ctx, opts.GatewayURL, gwClient)
		if gwErr != nil {
			fmt.Fprintf(stderr, "status: probe gateway catalog at %s: %v\n", opts.GatewayURL, gwErr.Err)
			return nil, int(gwErr.Kind)
		}
		if gwRes.CatalogStatus != "VERIFIED" {
			fmt.Fprintf(stderr, "status: gateway catalog not verified\n")
			return nil, ExitCodeValidation
		}
	}

	// 4. Re-inspect lease directly from live filesystem (never trust old observedState in manifest)
	liveLease, err := recovery.InspectLease(
		manifest.Lease.WorkspaceRoot,
		manifest.Lease.RunOwner,
		manifest.Lease.TaskID,
		manifest.Lease.OwnerID,
	)
	if err != nil {
		fmt.Fprintf(stderr, "status: re-inspect lease: %v\n", err)
		return nil, ExitCodeValidation
	}

	report := &StatusReport{
		Status:          "OK",
		RunID:           manifest.RunID,
		RunSpecSha256:   manifest.RunSpecSha256,
		GeneratedAt:     manifest.GeneratedAt,
		Target:          manifest.Target,
		Endpoints:       manifest.Endpoints,
		SessionBinding:  manifest.SessionBinding,
		WorktreeBinding: manifest.WorktreeBinding,
		AttachedProfile: manifest.AttachedSessionProfile,
		LiveLease:       liveLease,
	}

	if opts.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		fmt.Fprintf(stdout, "Run ID: %s\n", report.RunID)
		fmt.Fprintf(stdout, "AO Endpoint: %s (Identity: %s, Discovery: %s, PID: %d, Status: %s)\n",
			report.Endpoints.AOURL, report.Endpoints.AOIdentity, report.Endpoints.AODiscoverySource,
			report.Endpoints.AOPid, report.Endpoints.AOPidStatus)
		fmt.Fprintf(stdout, "Gateway: %s (Catalog: %s, CredentialEligibility: %s)\n",
			report.Endpoints.GatewayProbe.URL, report.Endpoints.GatewayProbe.CatalogStatus,
			report.Endpoints.GatewayProbe.CredentialEligibility)
		fmt.Fprintf(stdout, "Session: %s (Project: %s, Branch: %s)\n",
			report.SessionBinding.SessionID, report.SessionBinding.ProjectID, report.SessionBinding.SessionBranch)
		fmt.Fprintf(stdout, "Worktree: %s (Branch: %s, Head: %s, Clean: %v)\n",
			report.WorktreeBinding.CanonicalPath, report.WorktreeBinding.WorktreeBranch,
			report.WorktreeBinding.Head, report.WorktreeBinding.IsClean)
		fmt.Fprintf(stdout, "Attached Profile: %s (%s/%s, Effort: %s)\n",
			report.AttachedProfile.Kind, report.AttachedProfile.Harness, report.AttachedProfile.Model,
			report.AttachedProfile.ReasoningEffort)
		fmt.Fprintf(stdout, "Live Lease State: %s (Owner: %s, PID: %d)\n",
			report.LiveLease.ObservedState, report.LiveLease.OwnerID, report.LiveLease.ObservedPID)
	}

	return report, ExitCodeSuccess
}
