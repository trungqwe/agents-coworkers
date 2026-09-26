package recovery

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	// Private keys (PEM blocks)
	rePrivateKey = regexp.MustCompile(`(?s)-----BEGIN (?:[A-Z0-9_-]+ )?PRIVATE KEY-----[\s\S]*?-----END (?:[A-Z0-9_-]+ )?PRIVATE KEY-----`)

	// JWT tokens (3 base64url segments separated by dots, starting with eyJ)
	reJWT = regexp.MustCompile(`\beyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\b`)

	// Bearer authorization tokens
	reBearer = regexp.MustCompile(`(?i)\b(bearer\s+)[a-zA-Z0-9_\-\.\:\/+=~]+`)

	// Specific API key patterns (OpenAI sk-..., GitHub ghp_..., Google AIza..., etc.)
	reAPIKey = regexp.MustCompile(`\b(?:sk-[a-zA-Z0-9_-]{10,}|gh[pousr]_[a-zA-Z0-9]{15,}|glpat-[a-zA-Z0-9_-]{15,}|xox[bpa]-[a-zA-Z0-9_-]{8,}|AIza[0-9A-Za-z-_]{20,})\b`)

	// Key-value sensitive pairs (e.g. api_key=..., token: "...", secret = ...)
	reKVSecret = regexp.MustCompile(`(?i)\b((?:api_?key|access_?token|auth_?token|secret_?key|client_?secret|private_?key|password|passwd|secret|token)\s*[:=]\s*)(?:"[^"]*"|'[^']*'|[^\s"',;}&]+)`)

	// Email addresses
	reEmail = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)

	// Credential locators (file paths / filenames of known sensitive credential files)
	reCredLocator = regexp.MustCompile(`(?i)(?:[a-zA-Z]:\\[^:\s"'<>]+|/(?:[^/\s"'<>]+/)+|\b)(?:id_rsa|id_dsa|id_ecdsa|id_ed25519|auth\.json|credentials\.json|\.netrc|\.pgpass|service[_-]account.*\.json|[a-zA-Z0-9._-]*\.pem|[a-zA-Z0-9._-]*\.key)\b`)
)

func redactJSONPayloads(s string) string {
	var result strings.Builder
	n := len(s)
	i := 0
	for i < n {
		if s[i] == '{' {
			depth := 0
			inString := false
			escaped := false
			end := -1
			for j := i; j < n; j++ {
				c := s[j]
				if inString {
					if escaped {
						escaped = false
					} else if c == '\\' {
						escaped = true
					} else if c == '"' {
						inString = false
					}
				} else {
					if c == '"' {
						inString = true
					} else if c == '{' {
						depth++
					} else if c == '}' {
						depth--
						if depth == 0 {
							end = j
							break
						}
					}
				}
			}
			if end != -1 {
				candidate := s[i : end+1]
				var js map[string]any
				if json.Unmarshal([]byte(candidate), &js) == nil {
					result.WriteString("[REDACTED_PAYLOAD]")
					i = end + 1
					continue
				}
			}
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}

// SanitizeError applies redaction rules to error strings, removing credential
// locators, emails, tokens, private keys, and raw upstream JSON payloads.
func SanitizeError(s string) string {
	if s == "" {
		return ""
	}
	out := s
	out = rePrivateKey.ReplaceAllString(out, "[REDACTED_PRIVATE_KEY]")
	out = reJWT.ReplaceAllString(out, "[REDACTED_TOKEN]")
	out = reBearer.ReplaceAllString(out, "${1}[REDACTED_TOKEN]")
	out = reAPIKey.ReplaceAllString(out, "[REDACTED_KEY]")
	out = reKVSecret.ReplaceAllString(out, "${1}[REDACTED]")
	out = reEmail.ReplaceAllString(out, "[REDACTED_EMAIL]")
	out = reCredLocator.ReplaceAllString(out, "[REDACTED_CREDENTIAL_LOCATOR]")
	out = redactJSONPayloads(out)
	return out
}

// IsRetryableState returns true if the state indicates a non-terminal, retryable or reconciling state.
func IsRetryableState(state TaskState) bool {
	switch state {
	case StateWaitingRetry, StateDeliveryUncertain, StatePending:
		return true
	default:
		return false
	}
}

// StatusReport holds structured, sanitized status information for a checkpoint.
type StatusReport struct {
	TaskID          string          `json:"taskId"`
	DeliveryID      string          `json:"deliveryId"`
	SessionID       string          `json:"sessionId"`
	State           TaskState       `json:"state"`
	Retryable       bool            `json:"retryable"`
	Terminal        bool            `json:"terminal"`
	SideEffect      SideEffectState `json:"sideEffect"`
	Attempts        int             `json:"attempts"`
	RetryBudget     int             `json:"retryBudget"`
	NextRetryAt     *string         `json:"nextRetryAt,omitempty"`
	LastObservedUTC string          `json:"lastObservedUtc"`
	LastErrorKind   string          `json:"lastErrorKind,omitempty"`
	GitHead         string          `json:"gitHead"`
	ArtifactSHA256  string          `json:"artifactSha256"`
}

// FormatStatus formats a checkpoint into a human-readable text report with redaction applied.
func FormatStatus(cp Checkpoint) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Task ID:           %s\n", SanitizeError(cp.TaskID)))
	sb.WriteString(fmt.Sprintf("Delivery ID:       %s\n", SanitizeError(cp.DeliveryID)))
	sb.WriteString(fmt.Sprintf("Session ID:        %s\n", SanitizeError(cp.SessionID)))
	sb.WriteString(fmt.Sprintf("State:             %s\n", cp.State))
	sb.WriteString(fmt.Sprintf("Retryable:         %t\n", IsRetryableState(cp.State)))
	sb.WriteString(fmt.Sprintf("Terminal:          %t\n", cp.Terminal()))
	sb.WriteString(fmt.Sprintf("Side Effect:       %s\n", cp.SideEffect))
	sb.WriteString(fmt.Sprintf("Attempts:          %d/%d\n", cp.Attempts, cp.RetryBudget))
	if cp.NextRetryAt != nil {
		sb.WriteString(fmt.Sprintf("Next Retry At:     %s\n", cp.NextRetryAt.UTC().Format(time.RFC3339)))
	} else {
		sb.WriteString("Next Retry At:     none\n")
	}
	sb.WriteString(fmt.Sprintf("Last Observed UTC: %s\n", cp.LastObservedUTC.UTC().Format(time.RFC3339)))
	if cp.LastErrorKind != "" {
		sb.WriteString(fmt.Sprintf("Last Error Kind:   %s\n", SanitizeError(cp.LastErrorKind)))
	} else {
		sb.WriteString("Last Error Kind:   none\n")
	}
	sb.WriteString(fmt.Sprintf("Git Head:          %s\n", SanitizeError(cp.GitHead)))
	sb.WriteString(fmt.Sprintf("Artifact SHA256:   %s", SanitizeError(cp.ArtifactSHA256)))
	return sb.String()
}

// FormatStatusJSON formats a checkpoint into an indented JSON report with redaction applied.
func FormatStatusJSON(cp Checkpoint) (string, error) {
	var nextRetryStr *string
	if cp.NextRetryAt != nil {
		s := cp.NextRetryAt.UTC().Format(time.RFC3339)
		nextRetryStr = &s
	}

	report := StatusReport{
		TaskID:          SanitizeError(cp.TaskID),
		DeliveryID:      SanitizeError(cp.DeliveryID),
		SessionID:       SanitizeError(cp.SessionID),
		State:           cp.State,
		Retryable:       IsRetryableState(cp.State),
		Terminal:        cp.Terminal(),
		SideEffect:      cp.SideEffect,
		Attempts:        cp.Attempts,
		RetryBudget:     cp.RetryBudget,
		NextRetryAt:     nextRetryStr,
		LastObservedUTC: cp.LastObservedUTC.UTC().Format(time.RFC3339),
		LastErrorKind:   SanitizeError(cp.LastErrorKind),
		GitHead:         SanitizeError(cp.GitHead),
		ArtifactSHA256:  SanitizeError(cp.ArtifactSHA256),
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal status JSON: %w", err)
	}
	return string(data), nil
}
