package recovery

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAOHTTPRejectsNonLoopback(t *testing.T) {
	if _, err := NewAOHTTPClient("https://example.com", nil); err == nil {
		t.Fatal("non-loopback AO URL was accepted")
	}
}

func TestAOHTTPRecoverOnlyUsesStableDeliveryID(t *testing.T) {
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"outcome":"sent","turnId":"turn-1","state":"running"}`)
	}))
	defer server.Close()
	client, err := NewAOHTTPClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := client.RecoverOnly(context.Background(), "session-1", "delivery-1")
	if err != nil || receipt.TurnID != "turn-1" || receipt.State != DeliveryAccepted {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	if !strings.Contains(body, `"clientMessageId":"delivery-1"`) || !strings.Contains(body, `"recoverOnly":true`) || strings.Contains(body, `"text"`) {
		t.Fatalf("unexpected recover-only body: %s", body)
	}
}

func TestAOHTTPSentAndSteeredOutcomesAreDistinct(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantErr    error
	}{
		{name: "sent", body: `{"outcome":"sent","turnId":"turn-1","state":"running"}`},
		{name: "steered", body: `{"outcome":"steered","providerTurnId":"provider-1","state":"running"}`, wantErr: ErrSessionContaminated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer s.Close()
			c, _ := NewAOHTTPClient(s.URL, s.Client())
			got, err := c.Send(context.Background(), "session-1", "delivery-1", "task")
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("receipt=%+v err=%v want=%v", got, err, tc.wantErr)
			}
			if tc.wantErr == nil && got.TurnID != "turn-1" {
				t.Fatalf("receipt=%+v", got)
			}
		})
	}
}

func TestAOHTTPObservePaginatesUntilExactTurn(t *testing.T) {
	var pages int
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/sessions/session-1") {
			_, _ = io.WriteString(w, `{"id":"session-1"}`)
			return
		}
		pages++
		if r.URL.Query().Get("beforeSequence") == "50" {
			_, _ = io.WriteString(w, `{"oldestSequence":1,"hasMoreBefore":false,"turns":[{"id":"turn-old","state":"completed"}],"messages":[{"turnId":"turn-old","role":"assistant","text":"TASK_RECEIPT {\"taskId\":\"fixture-1\",\"artifactSha256\":\"abc\",\"accepted\":true}"}]}`)
			return
		}
		_, _ = io.WriteString(w, `{"oldestSequence":50,"hasMoreBefore":true,"turns":[{"id":"turn-new","state":"completed"}]}`)
	}))
	defer s.Close()
	c, _ := NewAOHTTPClient(s.URL, s.Client())
	got, err := c.Observe(context.Background(), "session-1", "turn-old")
	if err != nil || pages != 2 || got.Receipt == nil || got.Receipt.TaskID != "fixture-1" {
		t.Fatalf("pages=%d got=%+v err=%v", pages, got, err)
	}
}

func TestAOHTTPPreflightRequiresIdleOwnedSession(t *testing.T) {
	controller := "ready"
	sessionFixture, err := os.ReadFile(filepath.Join("testdata", "ao-session-response.json"))
	if err != nil {
		t.Fatal(err)
	}
	conversationFixture, err := os.ReadFile(filepath.Join("testdata", "ao-conversation-ready.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/conversation") {
			_, _ = w.Write([]byte(strings.Replace(string(conversationFixture), `"controller": "ready"`, `"controller": "`+controller+`"`, 1)))
			return
		}
		_, _ = w.Write(sessionFixture)
	}))
	defer s.Close()
	c, _ := NewAOHTTPClient(s.URL, s.Client())
	exp := SessionExpectation{SessionID: "session-1", ProjectID: "fixture", Kind: "worker", Harness: "codex", Model: "fixture-model", Effort: "high", Branch: "ao/session-1/root", Exclusive: true}
	if err := c.Preflight(context.Background(), exp); err != nil {
		t.Fatal(err)
	}
	controller = "busy"
	if err := c.Preflight(context.Background(), exp); !errors.Is(err, ErrSessionNotIdle) {
		t.Fatalf("err=%v", err)
	}
}

func TestAOHTTPPreflightFailsClosedOnIncompleteAuthoritativeReadback(t *testing.T) {
	sessionFixture, err := os.ReadFile(filepath.Join("testdata", "ao-session-response.json"))
	if err != nil {
		t.Fatal(err)
	}
	conversationFixture, err := os.ReadFile(filepath.Join("testdata", "ao-conversation-ready.json"))
	if err != nil {
		t.Fatal(err)
	}
	exp := SessionExpectation{SessionID: "session-1", ProjectID: "fixture", Kind: "worker", Harness: "codex", Model: "fixture-model", Effort: "high", Branch: "ao/session-1/root", Exclusive: true}
	cases := []struct {
		name, old    string
		conversation bool
	}{
		{"session_id", `"id": "session-1",`, false}, {"project", `"projectId": "fixture",`, false},
		{"kind", `"kind": "worker",`, false}, {"harness", `"harness": "codex",`, false},
		{"model", `"model": "fixture-model",`, false}, {"branch", `"branch": "ao/session-1/root",`, false},
		{"controller", `"controller": "ready",`, true}, {"effort", `"reasoningEffort": "high"`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sessionBody, conversationBody := string(sessionFixture), string(conversationFixture)
			if tc.conversation {
				conversationBody = strings.Replace(conversationBody, tc.old, "", 1)
			} else {
				sessionBody = strings.Replace(sessionBody, tc.old, "", 1)
			}
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/conversation") {
					_, _ = io.WriteString(w, conversationBody)
				} else {
					_, _ = io.WriteString(w, sessionBody)
				}
			}))
			defer s.Close()
			c, _ := NewAOHTTPClient(s.URL, s.Client())
			if err := c.Preflight(context.Background(), exp); err == nil {
				t.Fatal("incomplete readback was accepted")
			}
		})
	}
}

func TestAOHTTPStopOnlyInterruptsTheRecoveredActiveTurn(t *testing.T) {
	for _, tc := range []struct {
		name, turns string
		wantErr     bool
	}{
		{"owned", `[{"id":"turn-owned","state":"running"}]`, false},
		{"ambiguous", `[{"id":"turn-owned","state":"running"},{"id":"turn-other","state":"running"}]`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/sessions/session-1"):
					_, _ = io.WriteString(w, `{"session":{"id":"session-1"}}`)
				case r.Method == http.MethodGet:
					_, _ = io.WriteString(w, `{"hasMoreBefore":false,"turns":`+tc.turns+`}`)
				case r.Method == http.MethodPost:
					posts++
					w.WriteHeader(http.StatusNoContent)
				}
			}))
			defer s.Close()
			c, _ := NewAOHTTPClient(s.URL, s.Client())
			err := c.Stop(context.Background(), "session-1", "turn-owned")
			if (err != nil) != tc.wantErr || (!tc.wantErr && posts != 1) || (tc.wantErr && posts != 0) {
				t.Fatalf("err=%v posts=%d", err, posts)
			}
		})
	}
}

func TestAOHTTPObserveSeparatesTurnCompletionFromTaskReceipt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/sessions/session-1":
			_, _ = io.WriteString(w, `{"id":"session-1","isTerminated":false}`)
		case "/api/v1/sessions/session-1/conversation":
			_, _ = io.WriteString(w, `{"hasMoreBefore":false,"turns":[{"id":"turn-1","state":"completed"}],"messages":[{"turnId":"turn-1","role":"assistant","text":"done\nTASK_RECEIPT {\"taskId\":\"fixture-1\",\"artifactSha256\":\"abc\",\"accepted\":true}"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewAOHTTPClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	obs, err := client.Observe(context.Background(), "session-1", "turn-1")
	if err != nil || obs.Delivery != DeliveryCompleted || obs.Receipt == nil || obs.Receipt.TaskID != "fixture-1" || obs.Receipt.ArtifactSHA256 != "abc" || !obs.Receipt.Accepted {
		t.Fatalf("observation=%+v err=%v", obs, err)
	}
}

func TestAOHTTPTransportFailureRemainsUncertainAcrossRestart(t *testing.T) {
	var mu sync.Mutex
	var postBodies []string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/sessions/session-1") {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"session":{"id":"session-1","projectId":"fixture","kind":"worker","harness":"codex","model":"fixture-model","branch":"ao/session-1/root","isTerminated":false}}`)), Header: make(http.Header)}, nil
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/conversation") {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"controller":"ready","hasMoreBefore":false,"settings":{"model":"fixture-model","reasoningEffort":"high"}}`)), Header: make(http.Header)}, nil
		}
		if r.Method == http.MethodPost {
			data, _ := io.ReadAll(r.Body)
			mu.Lock()
			postBodies = append(postBodies, string(data))
			mu.Unlock()
			return nil, errors.New("simulated transport failure")
		}
		return nil, errors.New("unexpected request")
	})
	client, err := NewAOHTTPClient("http://127.0.0.1:3001", &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	cp := testCheckpoint()
	cp.ProjectID = "fixture"
	cp.SessionKind = "worker"
	cp.SessionHarness = "codex"
	cp.SessionModel = "fixture-model"
	cp.SessionEffort = "high"
	cp.SessionBranch = "ao/session-1/root"
	store := fileStore(t, cp)
	now := time.Date(2026, 9, 25, 3, 0, 0, 0, time.UTC)
	d := dispatcher(store, client, now)
	got, err := d.Step(context.Background())
	if err == nil || got.State != StateDeliveryUncertain || got.NextRetryAt == nil {
		t.Fatalf("first step got=%+v err=%v", got, err)
	}

	// Restart at the deadline. The second POST is recover-only with the same ID.
	d = dispatcher(store, client, *got.NextRetryAt)
	got, err = d.Step(context.Background())
	if err == nil || got.State != StateDeliveryUncertain {
		t.Fatalf("recovery got=%+v err=%v", got, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(postBodies) != 2 || strings.Contains(postBodies[0], `"recoverOnly":true`) || !strings.Contains(postBodies[1], `"recoverOnly":true`) || !strings.Contains(postBodies[1], `"clientMessageId":"delivery-1"`) {
		t.Fatalf("POST bodies=%v", postBodies)
	}
}

func TestAOHTTPAcceptanceResponseLostRestartRecoversSameTaskWithoutResend(t *testing.T) {
	var mu sync.Mutex
	ordinary, recoveries := 0, 0
	accepted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/sessions/session-1"):
			_, _ = io.WriteString(w, `{"session":{"id":"session-1","projectId":"fixture","kind":"worker","harness":"codex","model":"fixture-model","branch":"ao/session-1/root","isTerminated":false}}`)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/conversation"):
			if accepted {
				_, _ = io.WriteString(w, `{"controller":"ready","hasMoreBefore":false,"turns":[{"id":"turn-1","state":"completed"}],"messages":[{"turnId":"turn-1","role":"assistant","text":"TASK_RECEIPT {\"taskId\":\"fixture-1\",\"artifactSha256\":\"abc\",\"accepted\":true}"}]}`)
				return
			}
			_, _ = io.WriteString(w, `{"controller":"ready","hasMoreBefore":false,"settings":{"model":"fixture-model","reasoningEffort":"high"}}`)
		case r.Method == http.MethodPost:
			data, _ := io.ReadAll(r.Body)
			if strings.Contains(string(data), `"recoverOnly":true`) {
				recoveries++
			} else {
				ordinary++
				accepted = true
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"outcome":"sent","turnId":"turn-1","state":"completed","duplicate":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	base := server.Client().Transport
	dropOnce := true
	client, _ := NewAOHTTPClient(server.URL, &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		resp, err := base.RoundTrip(r)
		if err != nil {
			return nil, err
		}
		if r.Method == http.MethodPost && dropOnce {
			dropOnce = false
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			return nil, errors.New("simulated response loss after AO acceptance")
		}
		return resp, nil
	})})
	cp := testCheckpoint()
	cp.ProjectID = "fixture"
	cp.SessionKind = "worker"
	cp.SessionHarness = "codex"
	cp.SessionModel = "fixture-model"
	cp.SessionEffort = "high"
	cp.SessionBranch = "ao/session-1/root"
	store := fileStore(t, cp)
	now := time.Date(2026, 9, 25, 6, 0, 0, 0, time.UTC)
	got, err := dispatcher(store, client, now).Step(context.Background())
	if err == nil || got.State != StateDeliveryUncertain {
		t.Fatalf("first got=%+v err=%v", got, err)
	}
	got, err = dispatcher(store, client, *got.NextRetryAt).Step(context.Background())
	if err != nil || got.State != StateDelivered || got.AOTurnID != "turn-1" {
		t.Fatalf("recover got=%+v err=%v", got, err)
	}
	got, err = dispatcher(store, client, now.Add(2*time.Minute)).Step(context.Background())
	if err != nil || got.State != StateCompleted || got.SideEffect != SideEffectCompleted {
		t.Fatalf("receipt got=%+v err=%v", got, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if ordinary != 1 || recoveries != 1 {
		t.Fatalf("ordinary=%d recoveries=%d", ordinary, recoveries)
	}
}

func TestAOHTTPDefinitiveValidationFailureAllowsBoundedResend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	client, err := NewAOHTTPClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Send(context.Background(), "session-1", "delivery-1", "fixture")
	if !errors.Is(err, ErrRequestNotAccepted) {
		t.Fatalf("err=%v, want ErrRequestNotAccepted", err)
	}
}

func TestAOTurnStateMapping(t *testing.T) {
	tests := map[string]DeliveryState{
		"queued": DeliveryAccepted, "running": DeliveryAccepted, "recovered": DeliveryAccepted,
		"completed": DeliveryCompleted,
		"failed":    DeliveryRejected, "cancelled": DeliveryStopped, "interrupted": DeliveryStopped,
		"unknown": DeliveryNotFound,
	}
	for input, want := range tests {
		if got := mapTurnState(input); got != want {
			t.Errorf("mapTurnState(%q)=%q want %q", input, got, want)
		}
	}
}
