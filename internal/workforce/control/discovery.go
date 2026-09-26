package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const DefaultGatewayURL = "http://127.0.0.1:8317"

type runningJSON struct {
	PID       int       `json:"pid"`
	Port      int       `json:"port"`
	StartedAt time.Time `json:"startedAt"`
}

type aoIdentityResponse struct {
	HostID     string `json:"hostId"`
	APIVersion int    `json:"apiVersion"`
}

type openAIModelList struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// ValidateLoopbackURL validates that a URL is strictly loopback, http/https, with no userinfo,
// query parameters, fragment, or path beyond "/".
func ValidateLoopbackURL(rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return errors.New("URL is empty")
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid URL syntax: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("invalid URL scheme %q; only http or https allowed", u.Scheme)
	}
	if u.User != nil {
		return errors.New("URL must not contain userinfo")
	}
	if u.RawQuery != "" {
		return errors.New("URL must not contain query parameters")
	}
	if u.Fragment != "" {
		return errors.New("URL must not contain fragment")
	}
	path := u.Path
	if path != "" && path != "/" {
		return fmt.Errorf("URL must not contain base path beyond '/'; got %q", path)
	}
	host := strings.ToLower(u.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" && host != "[::1]" {
		return fmt.Errorf("host %q is not a loopback address (allowed: localhost, 127.0.0.1, ::1)", u.Hostname())
	}
	return nil
}

func IsLoopbackURL(rawURL string) bool {
	return ValidateLoopbackURL(rawURL) == nil
}

// CheckProcessAlive checks whether an OS process ID is currently running without using third-party libraries.
func CheckProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
		const STILL_ACTIVE = 259
		h, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
		if err != nil {
			return false
		}
		defer syscall.CloseHandle(h)
		var code uint32
		if err := syscall.GetExitCodeProcess(h, &code); err != nil {
			return false
		}
		return code == STILL_ACTIVE
	}
	// Unix-like systems: signal 0
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil
}

// ProbeAOIdentity sends GET /api/v1/identity to the given base URL.
// It returns (hostId, *DiscoveryError).
func ProbeAOIdentity(ctx context.Context, client *http.Client, baseURL string) (string, *DiscoveryError) {
	u := strings.TrimRight(baseURL, "/") + "/api/v1/identity"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", &DiscoveryError{Kind: ErrKindValidation, Err: err}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", &DiscoveryError{Kind: ErrKindUnavailable, Err: fmt.Errorf("AO identity probe connection failed: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &DiscoveryError{
			Kind: ErrKindValidation,
			Err:  fmt.Errorf("AO identity probe returned HTTP %d", resp.StatusCode),
		}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<18))
	if err != nil {
		return "", &DiscoveryError{Kind: ErrKindValidation, Err: fmt.Errorf("read AO identity body: %w", err)}
	}
	var res aoIdentityResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return "", &DiscoveryError{Kind: ErrKindValidation, Err: fmt.Errorf("decode AO identity JSON: %w", err)}
	}
	if strings.TrimSpace(res.HostID) == "" {
		return "", &DiscoveryError{Kind: ErrKindValidation, Err: errors.New("AO identity probe returned empty hostId")}
	}
	return res.HostID, nil
}

// ResolveAOEndpoint discovers the active AO daemon URL and verifies its identity and PID status.
func ResolveAOEndpoint(ctx context.Context, explicitURL string, client *http.Client) (*AODiscoveryResult, *DiscoveryError) {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	// 1. Explicit --ao-url flag
	if explicitURL != "" {
		if err := ValidateLoopbackURL(explicitURL); err != nil {
			return nil, &DiscoveryError{Kind: ErrKindValidation, Err: fmt.Errorf("explicit --ao-url %q invalid: %w", explicitURL, err)}
		}
		hostID, dErr := ProbeAOIdentity(ctx, client, explicitURL)
		if dErr != nil {
			return nil, dErr
		}
		return &AODiscoveryResult{
			URL:             strings.TrimRight(explicitURL, "/"),
			Identity:        hostID,
			DiscoverySource: "explicit_url",
			PID:             0,
			PIDStatus:       "NOT_OBSERVED",
		}, nil
	}

	// 2. Environment variable AO_BASE_URL
	if envURL := os.Getenv("AO_BASE_URL"); envURL != "" {
		if err := ValidateLoopbackURL(envURL); err != nil {
			return nil, &DiscoveryError{Kind: ErrKindValidation, Err: fmt.Errorf("AO_BASE_URL %q invalid: %w", envURL, err)}
		}
		hostID, dErr := ProbeAOIdentity(ctx, client, envURL)
		if dErr != nil {
			return nil, dErr
		}
		return &AODiscoveryResult{
			URL:             strings.TrimRight(envURL, "/"),
			Identity:        hostID,
			DiscoverySource: "environment_url",
			PID:             0,
			PIDStatus:       "NOT_OBSERVED",
		}, nil
	}

	// 3. Environment variable AO_RUN_FILE
	if runFile := os.Getenv("AO_RUN_FILE"); runFile != "" {
		res, dErr := resolveRunFile(ctx, client, runFile)
		if dErr != nil {
			return nil, dErr
		}
		return res, nil
	}

	// 4. Default candidate running.json files:
	// Only ~/.ao/dev/running.json and ~/.ao/running.json
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, &DiscoveryError{Kind: ErrKindUnavailable, Err: errors.New("cannot determine user home directory for AO candidate discovery")}
	}
	rawCandidates := []string{
		filepath.Join(home, ".ao", "dev", "running.json"),
		filepath.Join(home, ".ao", "running.json"),
	}

	// Dedupe canonical candidate paths using EvalSymlinks / filepath.Abs
	seenPaths := make(map[string]bool)
	var candidatePaths []string
	for _, p := range rawCandidates {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = filepath.Clean(p)
		}
		if eval, err := filepath.EvalSymlinks(abs); err == nil {
			abs = eval
		}
		norm := abs
		if runtime.GOOS == "windows" {
			norm = strings.ToLower(norm)
		}
		if !seenPaths[norm] {
			seenPaths[norm] = true
			candidatePaths = append(candidatePaths, abs)
		}
	}

	var validResults []*AODiscoveryResult
	anyCandidateExists := false

	for _, p := range candidatePaths {
		if _, statErr := os.Stat(p); statErr == nil {
			anyCandidateExists = true
			res, dErr := resolveRunFile(ctx, client, p)
			if dErr != nil {
				// Candidate exists but is invalid (JSON, schema, PID, identity probe): validation error, exit 2
				return nil, &DiscoveryError{
					Kind: ErrKindValidation,
					Err:  fmt.Errorf("candidate file %s is invalid: %w", p, dErr.Err),
				}
			}
			validResults = append(validResults, res)
		}
	}

	if !anyCandidateExists {
		return nil, &DiscoveryError{Kind: ErrKindUnavailable, Err: errors.New("no AO running.json candidate files exist")}
	}
	if len(validResults) == 0 {
		return nil, &DiscoveryError{Kind: ErrKindUnavailable, Err: errors.New("no valid AO daemon found in default candidate files")}
	}

	// Dedupe results by daemon identity: hostId + PID + resolved URL/port
	type daemonKey struct {
		hostID string
		pid    int
		url    string
	}
	daemonMap := make(map[daemonKey]*AODiscoveryResult)
	for _, res := range validResults {
		key := daemonKey{
			hostID: res.Identity,
			pid:    res.PID,
			url:    res.URL,
		}
		daemonMap[key] = res
	}

	if len(daemonMap) > 1 {
		return nil, &DiscoveryError{Kind: ErrKindValidation, Err: errors.New("multiple valid AO daemons found; please specify --ao-url explicitly")}
	}

	for _, res := range daemonMap {
		return res, nil
	}

	return nil, &DiscoveryError{Kind: ErrKindUnavailable, Err: errors.New("no valid AO daemon discovered")}
}

func resolveRunFile(ctx context.Context, client *http.Client, runFilePath string) (*AODiscoveryResult, *DiscoveryError) {
	data, err := os.ReadFile(runFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &DiscoveryError{Kind: ErrKindUnavailable, Err: fmt.Errorf("run file %s does not exist", runFilePath)}
		}
		return nil, &DiscoveryError{Kind: ErrKindValidation, Err: fmt.Errorf("read run file %s: %w", runFilePath, err)}
	}
	var r runningJSON
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, &DiscoveryError{Kind: ErrKindValidation, Err: fmt.Errorf("parse running.json: %w", err)}
	}
	if r.Port <= 0 || r.PID <= 0 {
		return nil, &DiscoveryError{Kind: ErrKindValidation, Err: fmt.Errorf("running.json has invalid port=%d or pid=%d", r.Port, r.PID)}
	}
	if !CheckProcessAlive(r.PID) {
		return nil, &DiscoveryError{Kind: ErrKindValidation, Err: fmt.Errorf("daemon process PID %d is not alive", r.PID)}
	}

	loopbackURL := fmt.Sprintf("http://127.0.0.1:%d", r.Port)
	hostID, dErr := ProbeAOIdentity(ctx, client, loopbackURL)
	if dErr != nil {
		return nil, &DiscoveryError{Kind: dErr.Kind, Err: fmt.Errorf("daemon at %s failed identity probe: %w", loopbackURL, dErr.Err)}
	}

	return &AODiscoveryResult{
		URL:             loopbackURL,
		Identity:        hostID,
		DiscoverySource: "run_file",
		PID:             r.PID,
		PIDStatus:       "VERIFIED",
	}, nil
}

// ResolveGatewayEndpoint discovers the active CLIProxyAPI endpoint and verifies its model catalog.
func ResolveGatewayEndpoint(ctx context.Context, explicitURL string, client *http.Client) (*GatewayResult, *DiscoveryError) {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	gwURL := strings.TrimSpace(explicitURL)
	if gwURL == "" {
		gwURL = strings.TrimSpace(os.Getenv("COWORKERS_GATEWAY_URL"))
	}
	if gwURL == "" {
		gwURL = DefaultGatewayURL
	}

	if err := ValidateLoopbackURL(gwURL); err != nil {
		return nil, &DiscoveryError{Kind: ErrKindValidation, Err: fmt.Errorf("gateway URL %q invalid: %w", gwURL, err)}
	}

	gwURL = strings.TrimRight(gwURL, "/")
	reqURL := gwURL + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, &DiscoveryError{Kind: ErrKindValidation, Err: err}
	}

	key := strings.TrimSpace(os.Getenv("CLIPROXY_KEY"))
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, &DiscoveryError{Kind: ErrKindUnavailable, Err: fmt.Errorf("gateway unreachable: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, &DiscoveryError{
			Kind: ErrKindValidation,
			Err:  fmt.Errorf("gateway authentication failed: HTTP %d (check CLIPROXY_KEY)", resp.StatusCode),
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &DiscoveryError{
			Kind: ErrKindValidation,
			Err:  fmt.Errorf("gateway catalog returned HTTP %d", resp.StatusCode),
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, &DiscoveryError{Kind: ErrKindValidation, Err: fmt.Errorf("read gateway catalog body: %w", err)}
	}

	var list openAIModelList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, &DiscoveryError{Kind: ErrKindValidation, Err: fmt.Errorf("decode gateway model catalog JSON: %w", err)}
	}

	var models []string
	for _, m := range list.Data {
		if id := strings.TrimSpace(m.ID); id != "" {
			models = append(models, id)
		}
	}

	if len(models) == 0 {
		return nil, &DiscoveryError{Kind: ErrKindValidation, Err: errors.New("gateway model catalog is empty")}
	}

	return &GatewayResult{
		URL:                   gwURL,
		CatalogStatus:         "VERIFIED",
		ObservedModels:        models,
		ProviderCallPerformed: false,
		CredentialEligibility: "NOT_OBSERVED",
	}, nil
}
