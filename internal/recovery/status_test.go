package recovery

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestStatusFormattingAcrossStates(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	nextRetry := now.Add(5 * time.Minute)

	tests := []struct {
		name            string
		cp              Checkpoint
		expectState     TaskState
		expectRetryable bool
		expectTerminal  bool
		expectNextRetry string
	}{
		{
			name: "COMPLETED",
			cp: Checkpoint{
				Version:         1,
				TaskID:          "task-001",
				DeliveryID:      "del-001",
				SessionID:       "sess-001",
				State:           StateCompleted,
				SideEffect:      SideEffectCompleted,
				Attempts:        1,
				RetryBudget:     3,
				LastObservedUTC: now,
				GitHead:         "4dfbf9905ff12c363307e5974fab719a0911b114",
				ArtifactSHA256:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
			expectState:     StateCompleted,
			expectRetryable: false,
			expectTerminal:  true,
			expectNextRetry: "none",
		},
		{
			name: "WAITING_RETRY",
			cp: Checkpoint{
				Version:         1,
				TaskID:          "task-002",
				DeliveryID:      "del-002",
				SessionID:       "sess-002",
				State:           StateWaitingRetry,
				SideEffect:      SideEffectPending,
				Attempts:        2,
				RetryBudget:     5,
				NextRetryAt:     &nextRetry,
				LastErrorKind:   "PROVIDER_RATE_LIMITED",
				LastObservedUTC: now,
				GitHead:         "4dfbf9905ff12c363307e5974fab719a0911b114",
				ArtifactSHA256:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
			expectState:     StateWaitingRetry,
			expectRetryable: true,
			expectTerminal:  false,
			expectNextRetry: nextRetry.Format(time.RFC3339),
		},
		{
			name: "BLOCKED",
			cp: Checkpoint{
				Version:         1,
				TaskID:          "task-003",
				DeliveryID:      "del-003",
				SessionID:       "sess-003",
				State:           StateBlocked,
				SideEffect:      SideEffectPending,
				Attempts:        5,
				RetryBudget:     5,
				LastErrorKind:   "BLOCKED_PROFILE_POOL_UNAVAILABLE_GEMINI_3_8_FLASH_HIGH",
				LastObservedUTC: now,
				GitHead:         "4dfbf9905ff12c363307e5974fab719a0911b114",
				ArtifactSHA256:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
			expectState:     StateBlocked,
			expectRetryable: false,
			expectTerminal:  true,
			expectNextRetry: "none",
		},
		{
			name: "DELIVERY_UNCERTAIN",
			cp: Checkpoint{
				Version:         1,
				TaskID:          "task-004",
				DeliveryID:      "del-004",
				SessionID:       "sess-004",
				State:           StateDeliveryUncertain,
				SideEffect:      SideEffectPending,
				Attempts:        1,
				RetryBudget:     3,
				NextRetryAt:     &nextRetry,
				LastErrorKind:   "DELIVERY_UNCERTAIN",
				LastObservedUTC: now,
				GitHead:         "4dfbf9905ff12c363307e5974fab719a0911b114",
				ArtifactSHA256:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
			expectState:     StateDeliveryUncertain,
			expectRetryable: true,
			expectTerminal:  false,
			expectNextRetry: nextRetry.Format(time.RFC3339),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			textOut := FormatStatus(tc.cp)
			if !strings.Contains(textOut, "Task ID:           "+tc.cp.TaskID) {
				t.Errorf("text output missing Task ID: %s", textOut)
			}
			if !strings.Contains(textOut, "Delivery ID:       "+tc.cp.DeliveryID) {
				t.Errorf("text output missing Delivery ID: %s", textOut)
			}
			if !strings.Contains(textOut, "Session ID:        "+tc.cp.SessionID) {
				t.Errorf("text output missing Session ID: %s", textOut)
			}
			if !strings.Contains(textOut, "State:             "+string(tc.expectState)) {
				t.Errorf("text output missing State %s: %s", tc.expectState, textOut)
			}
			if tc.expectRetryable && !strings.Contains(textOut, "Retryable:         true") {
				t.Errorf("text output expected Retryable: true, got: %s", textOut)
			}
			if !tc.expectRetryable && !strings.Contains(textOut, "Retryable:         false") {
				t.Errorf("text output expected Retryable: false, got: %s", textOut)
			}
			if tc.expectTerminal && !strings.Contains(textOut, "Terminal:          true") {
				t.Errorf("text output expected Terminal: true, got: %s", textOut)
			}
			if !tc.expectTerminal && !strings.Contains(textOut, "Terminal:          false") {
				t.Errorf("text output expected Terminal: false, got: %s", textOut)
			}
			if !strings.Contains(textOut, "Git Head:          "+tc.cp.GitHead) {
				t.Errorf("text output missing Git Head: %s", textOut)
			}
			if !strings.Contains(textOut, "Artifact SHA256:   "+tc.cp.ArtifactSHA256) {
				t.Errorf("text output missing Artifact SHA256: %s", textOut)
			}
			if !strings.Contains(textOut, "Next Retry At:     "+tc.expectNextRetry) {
				t.Errorf("text output expected Next Retry At %s, got: %s", tc.expectNextRetry, textOut)
			}

			// Test JSON output
			jsonOut, err := FormatStatusJSON(tc.cp)
			if err != nil {
				t.Fatalf("FormatStatusJSON failed: %v", err)
			}
			var report StatusReport
			if err := json.Unmarshal([]byte(jsonOut), &report); err != nil {
				t.Fatalf("Unmarshal JSON output failed: %v", err)
			}
			if report.TaskID != tc.cp.TaskID {
				t.Errorf("JSON report TaskID = %s, want %s", report.TaskID, tc.cp.TaskID)
			}
			if report.DeliveryID != tc.cp.DeliveryID {
				t.Errorf("JSON report DeliveryID = %s, want %s", report.DeliveryID, tc.cp.DeliveryID)
			}
			if report.SessionID != tc.cp.SessionID {
				t.Errorf("JSON report SessionID = %s, want %s", report.SessionID, tc.cp.SessionID)
			}
			if report.State != tc.expectState {
				t.Errorf("JSON report State = %s, want %s", report.State, tc.expectState)
			}
			if report.Retryable != tc.expectRetryable {
				t.Errorf("JSON report Retryable = %v, want %v", report.Retryable, tc.expectRetryable)
			}
			if report.Terminal != tc.expectTerminal {
				t.Errorf("JSON report Terminal = %v, want %v", report.Terminal, tc.expectTerminal)
			}
			if report.GitHead != tc.cp.GitHead {
				t.Errorf("JSON report GitHead = %s, want %s", report.GitHead, tc.cp.GitHead)
			}
			if report.ArtifactSHA256 != tc.cp.ArtifactSHA256 {
				t.Errorf("JSON report ArtifactSHA256 = %s, want %s", report.ArtifactSHA256, tc.cp.ArtifactSHA256)
			}
		})
	}
}

func TestStatusRedactionContract(t *testing.T) {
	sensitiveErrors := []string{
		"failed with email alice@example.com on host",
		"auth failure: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.sflk-secret-signature",
		"upstream rejected sk-proj-1234567890abcdef1234567890",
		"github token ghp_1234567890abcdefghijklmnopqrstuvwxyz leaked",
		"google key AIzaSyD-1234567890abcdefghijklmnopqr invalid",
		"failed to read key file /home/runner/.ssh/id_rsa: permission denied",
		"failed to open C:\\Users\\Admin\\.config\\auth.json file",
		"failed to load /var/secrets/credentials.json for tenant",
		"kv secrets: api_key=secret-key-value-123 token=\"token-value-456\" password=my-super-secret-password",
		"-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA0...\n-----END RSA PRIVATE KEY-----",
		`upstream error HTTP 503: {"error": {"code": 503, "message": "sensitive internal trace", "details": "internal secret"}}`,
	}

	forbiddenSubstrings := []string{
		"alice@example.com",
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		"sflk-secret-signature",
		"sk-proj-1234567890abcdef1234567890",
		"ghp_1234567890abcdefghijklmnopqrstuvwxyz",
		"AIzaSyD-1234567890abcdefghijklmnopqr",
		"/home/runner/.ssh/id_rsa",
		"auth.json",
		"credentials.json",
		"secret-key-value-123",
		"token-value-456",
		"my-super-secret-password",
		"BEGIN RSA PRIVATE KEY",
		"MIIEowIBAAKCAQEA0",
		"sensitive internal trace",
	}

	for _, errMsg := range sensitiveErrors {
		cp := Checkpoint{
			Version:         1,
			TaskID:          "task-sec-test",
			DeliveryID:      "del-sec-test",
			SessionID:       "sess-sec-test",
			State:           StateBlocked,
			SideEffect:      SideEffectPending,
			Attempts:        1,
			RetryBudget:     3,
			LastErrorKind:   errMsg,
			LastObservedUTC: time.Now().UTC(),
			GitHead:         "4dfbf9905ff12c363307e5974fab719a0911b114",
			ArtifactSHA256:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		}

		textOut := FormatStatus(cp)
		jsonOut, err := FormatStatusJSON(cp)
		if err != nil {
			t.Fatalf("FormatStatusJSON failed: %v", err)
		}

		for _, forbidden := range forbiddenSubstrings {
			if strings.Contains(textOut, forbidden) {
				t.Errorf("FormatStatus leaked forbidden string %q: got %s", forbidden, textOut)
			}
			if strings.Contains(jsonOut, forbidden) {
				t.Errorf("FormatStatusJSON leaked forbidden string %q: got %s", forbidden, jsonOut)
			}
		}
	}
}

func TestStatusSanitizeError(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "clean error kind",
			input:    "BLOCKED_PROFILE_ELIGIBILITY_GPT55",
			expected: "BLOCKED_PROFILE_ELIGIBILITY_GPT55",
		},
		{
			name:     "clean pool unavailable",
			input:    "BLOCKED_PROFILE_POOL_UNAVAILABLE_GEMINI_3_8_FLASH_HIGH",
			expected: "BLOCKED_PROFILE_POOL_UNAVAILABLE_GEMINI_3_8_FLASH_HIGH",
		},
		{
			name:     "email redaction",
			input:    "contact developer@example.org for help",
			expected: "contact [REDACTED_EMAIL] for help",
		},
		{
			name:     "bearer token redaction",
			input:    "header Authorization: Bearer abc123def456xyz789",
			expected: "header Authorization: Bearer [REDACTED_TOKEN]",
		},
		{
			name:     "api key sk prefix",
			input:    "invalid api key sk-1234567890abcdef123456",
			expected: "invalid api key [REDACTED_KEY]",
		},
		{
			name:     "private key block",
			input:    "key loaded: -----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg...\n-----END PRIVATE KEY-----",
			expected: "key loaded: [REDACTED_PRIVATE_KEY]",
		},
		{
			name:     "credential file path",
			input:    "could not read /etc/secrets/auth.json: not found",
			expected: "could not read [REDACTED_CREDENTIAL_LOCATOR]: not found",
		},
		{
			name:     "raw upstream json payload",
			input:    `upstream 503: {"error": {"message": "internal failure", "code": 503}}`,
			expected: "upstream 503: [REDACTED_PAYLOAD]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeError(tc.input)
			if got != tc.expected {
				t.Errorf("SanitizeError(%q) = %q; want %q", tc.input, got, tc.expected)
			}
		})
	}
}
