package control

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDiscovery_IsLoopbackURL(t *testing.T) {
	for _, valid := range []string{
		"http://127.0.0.1:3000",
		"http://localhost:8317",
		"http://[::1]:8080",
		"http://127.0.0.1",
		"http://127.0.0.1/",
	} {
		if !IsLoopbackURL(valid) {
			t.Errorf("expected %s to be valid loopback URL", valid)
		}
	}

	for _, invalid := range []string{
		"http://192.168.1.5:3000",
		"http://example.com:8317",
		"http://10.0.0.1",
		"://bad-url",
		"http://user:pass@127.0.0.1:3000", // userinfo forbidden
		"http://127.0.0.1:3000/some/path", // base path beyond '/' forbidden
		"http://127.0.0.1:3000?query=1",   // query forbidden
		"http://127.0.0.1:3000#frag",      // fragment forbidden
		"ftp://127.0.0.1:3000",            // scheme non-http/https forbidden
	} {
		if IsLoopbackURL(invalid) {
			t.Errorf("expected %s to be invalid loopback URL", invalid)
		}
	}
}

func TestResolveAOEndpoint_ExplicitURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/identity" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-test-host", "apiVersion": 1})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	ctx := context.Background()
	res, dErr := ResolveAOEndpoint(ctx, ts.URL, ts.Client())
	if dErr != nil {
		t.Fatalf("ResolveAOEndpoint failed: %v", dErr.Err)
	}
	if res.Identity != "ao-test-host" {
		t.Errorf("expected identity ao-test-host, got %s", res.Identity)
	}
	if res.DiscoverySource != "explicit_url" {
		t.Errorf("expected discoverySource explicit_url, got %s", res.DiscoverySource)
	}
	if res.PID != 0 || res.PIDStatus != "NOT_OBSERVED" {
		t.Errorf("expected PID 0 and NOT_OBSERVED, got %d and %s", res.PID, res.PIDStatus)
	}
}

func TestResolveAOEndpoint_RunFile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/identity" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-runfile-host", "apiVersion": 1})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	port, _ := strconv.Atoi(u.Port())
	pid := os.Getpid()

	tmpDir := t.TempDir()
	runFilePath := filepath.Join(tmpDir, "running.json")
	runData, _ := json.Marshal(map[string]any{
		"pid":  pid,
		"port": port,
	})
	_ = os.WriteFile(runFilePath, runData, 0o600)

	os.Setenv("AO_RUN_FILE", runFilePath)
	defer os.Unsetenv("AO_RUN_FILE")

	ctx := context.Background()
	res, dErr := ResolveAOEndpoint(ctx, "", ts.Client())
	if dErr != nil {
		t.Fatalf("ResolveAOEndpoint from run file failed: %v", dErr.Err)
	}
	if res.Identity != "ao-runfile-host" {
		t.Errorf("expected identity ao-runfile-host, got %s", res.Identity)
	}
	if res.DiscoverySource != "run_file" {
		t.Errorf("expected discoverySource run_file, got %s", res.DiscoverySource)
	}
	if res.PID != pid || res.PIDStatus != "VERIFIED" {
		t.Errorf("expected PID %d and VERIFIED, got %d and %s", pid, res.PID, res.PIDStatus)
	}
}

func TestResolveAOEndpoint_CandidateInvalidExists(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("AO_BASE_URL", "")
	t.Setenv("AO_RUN_FILE", "")

	aoDir := filepath.Join(tmpHome, ".ao")
	_ = os.MkdirAll(aoDir, 0o755)
	badCandidate := filepath.Join(aoDir, "running.json")
	_ = os.WriteFile(badCandidate, []byte(`{not-json}`), 0o644)

	ctx := context.Background()
	_, dErr := ResolveAOEndpoint(ctx, "", nil)
	if dErr == nil {
		t.Fatal("expected error on invalid candidate file, got nil")
	}
	if dErr.Kind != ErrKindValidation {
		t.Errorf("expected ErrKindValidation (2) on corrupt existing candidate, got %v", dErr.Kind)
	}
}

func TestResolveAOEndpoint_TwoCandidatesSameDaemonPass(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/identity" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "ao-same-daemon", "apiVersion": 1})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	port, _ := strconv.Atoi(u.Port())
	pid := os.Getpid()

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("AO_BASE_URL", "")
	t.Setenv("AO_RUN_FILE", "")

	aoDir := filepath.Join(tmpHome, ".ao")
	aoDevDir := filepath.Join(aoDir, "dev")
	_ = os.MkdirAll(aoDevDir, 0o755)

	runData, _ := json.Marshal(map[string]any{"pid": pid, "port": port})
	_ = os.WriteFile(filepath.Join(aoDir, "running.json"), runData, 0o644)
	_ = os.WriteFile(filepath.Join(aoDevDir, "running.json"), runData, 0o644)

	ctx := context.Background()
	res, dErr := ResolveAOEndpoint(ctx, "", ts.Client())
	if dErr != nil {
		t.Fatalf("expected two candidates pointing to same daemon to PASS, got error: %v", dErr.Err)
	}
	if res.Identity != "ao-same-daemon" {
		t.Errorf("expected identity ao-same-daemon, got %s", res.Identity)
	}
}

func TestResolveAOEndpoint_TwoCandidatesDifferentDaemonsFail(t *testing.T) {
	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "daemon-1", "apiVersion": 1})
	}))
	defer ts1.Close()

	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"hostId": "daemon-2", "apiVersion": 1})
	}))
	defer ts2.Close()

	u1, _ := url.Parse(ts1.URL)
	port1, _ := strconv.Atoi(u1.Port())
	u2, _ := url.Parse(ts2.URL)
	port2, _ := strconv.Atoi(u2.Port())
	pid := os.Getpid()

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("AO_BASE_URL", "")
	t.Setenv("AO_RUN_FILE", "")

	aoDir := filepath.Join(tmpHome, ".ao")
	aoDevDir := filepath.Join(aoDir, "dev")
	_ = os.MkdirAll(aoDevDir, 0o755)

	runData1, _ := json.Marshal(map[string]any{"pid": pid, "port": port1})
	runData2, _ := json.Marshal(map[string]any{"pid": pid, "port": port2})
	_ = os.WriteFile(filepath.Join(aoDir, "running.json"), runData1, 0o644)
	_ = os.WriteFile(filepath.Join(aoDevDir, "running.json"), runData2, 0o644)

	ctx := context.Background()
	_, dErr := ResolveAOEndpoint(ctx, "", nil)
	if dErr == nil {
		t.Fatal("expected error when candidates point to different daemons, got nil")
	}
	if dErr.Kind != ErrKindValidation {
		t.Errorf("expected ErrKindValidation (2) on multiple different daemons, got %v", dErr.Kind)
	}
}

func TestResolveGatewayEndpoint_ModelsAndAuth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer secret-key-123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "gpt-5"},
					{"id": "gemini-2.5"},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	ctx := context.Background()

	// 1. Without key -> 401 error (validation error kind)
	os.Unsetenv("CLIPROXY_KEY")
	_, dErr := ResolveGatewayEndpoint(ctx, ts.URL, ts.Client())
	if dErr == nil {
		t.Fatal("expected unauthorized error when key is absent, got nil")
	}
	if dErr.Kind != ErrKindValidation {
		t.Errorf("expected ErrKindValidation on 401, got %v", dErr.Kind)
	}

	// 2. With valid key -> success
	os.Setenv("CLIPROXY_KEY", "secret-key-123")
	defer os.Unsetenv("CLIPROXY_KEY")

	gwRes, dErr := ResolveGatewayEndpoint(ctx, ts.URL, ts.Client())
	if dErr != nil {
		t.Fatalf("ResolveGatewayEndpoint failed: %v", dErr.Err)
	}
	if len(gwRes.ObservedModels) != 2 || gwRes.ObservedModels[0] != "gpt-5" {
		t.Errorf("unexpected observed models: %v", gwRes.ObservedModels)
	}
	if gwRes.ProviderCallPerformed != false {
		t.Error("expected ProviderCallPerformed to be false")
	}
	if gwRes.CredentialEligibility != "NOT_OBSERVED" {
		t.Errorf("expected NOT_OBSERVED, got %s", gwRes.CredentialEligibility)
	}
}
