package control

import (
	"encoding/json"
	"strings"
	"testing"
)

func validSampleRunSpec() []byte {
	return []byte(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "run-20261001-sample-001",
  "description": "Sample test run spec",
  "target": {
    "targetRoot": ".",
    "executionWorkspace": ".",
    "baselineSha": "0123456789abcdef0123456789abcdef01234567",
    "expectedBranch": "feature/test"
  },
  "workspacePolicy": {
    "productRootMustBeClean": true,
    "executionWorkspaceDirtyPolicy": "require_clean"
  },
  "requiredTools": [
    {
      "name": "git",
      "required": true
    }
  ],
  "endpointPolicy": {
    "aoDiscovery": "flag_env_runfile_candidates_fail_closed",
    "gatewayDiscovery": "flag_env_historical_default_fail_closed"
  },
  "profiles": {
    "worker": {
      "model": "gpt-5",
      "reasoningEffort": "medium"
    }
  }
}`)
}

func TestParseAndValidateRunSpec_Valid(t *testing.T) {
	raw := validSampleRunSpec()
	spec, sha, err := ParseAndValidateRunSpec(raw)
	if err != nil {
		t.Fatalf("ParseAndValidateRunSpec failed: %v", err)
	}
	if spec.RunID != "run-20261001-sample-001" {
		t.Errorf("unexpected runId: %s", spec.RunID)
	}
	if sha == "" {
		t.Error("expected non-empty raw SHA")
	}
	if spec.WorkspacePolicy.ExecutionWorkspaceDirtyPolicy != "require_clean" {
		t.Errorf("unexpected dirty policy: %s", spec.WorkspacePolicy.ExecutionWorkspaceDirtyPolicy)
	}
}

func TestParseAndValidateRunSpec_TrailingTokensRejected(t *testing.T) {
	raw := string(validSampleRunSpec()) + ` {"trailing": true}`
	_, _, err := ParseAndValidateRunSpec([]byte(raw))
	if err == nil {
		t.Fatal("expected error on trailing tokens, got nil")
	}
	if !strings.Contains(err.Error(), "trailing") {
		t.Errorf("expected trailing token error, got: %v", err)
	}
}

func TestParseAndValidateRunSpec_DisallowUnknownFields(t *testing.T) {
	invalidJSON := []byte(`{
  "schemaVersion": "run-spec/v1-draft",
  "runId": "run-20261001-sample-001",
  "target": {
    "targetRoot": ".",
    "executionWorkspace": ".",
    "baselineSha": "0123456789abcdef0123456789abcdef01234567",
    "expectedBranch": "feature/test"
  },
  "workspacePolicy": {
    "productRootMustBeClean": true,
    "executionWorkspaceDirtyPolicy": "require_clean"
  },
  "requiredTools": [
    {
      "name": "git",
      "required": true
    }
  ],
  "endpointPolicy": {
    "aoDiscovery": "flag_env_runfile_candidates_fail_closed",
    "gatewayDiscovery": "flag_env_historical_default_fail_closed"
  },
  "profiles": {
    "worker": {
      "model": "gpt-5",
      "reasoningEffort": "medium"
    }
  },
  "unknownField": "forbidden"
}`)

	_, _, err := ParseAndValidateRunSpec(invalidJSON)
	if err == nil {
		t.Fatal("expected error on unknown field, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseAndValidateRunSpec_EndpointPolicyValues(t *testing.T) {
	// Bad aoDiscovery
	badAO := []byte(strings.Replace(string(validSampleRunSpec()), `"flag_env_runfile_candidates_fail_closed"`, `"invalid_ao"`, 1))
	if _, _, err := ParseAndValidateRunSpec(badAO); err == nil {
		t.Fatal("expected error on bad aoDiscovery, got nil")
	}

	// Bad gatewayDiscovery
	badGW := []byte(strings.Replace(string(validSampleRunSpec()), `"flag_env_historical_default_fail_closed"`, `"invalid_gw"`, 1))
	if _, _, err := ParseAndValidateRunSpec(badGW); err == nil {
		t.Fatal("expected error on bad gatewayDiscovery, got nil")
	}
}

func TestParseAndValidateRunSpec_ConcurrencyBounds(t *testing.T) {
	// Concurrency = 0 -> error
	raw0 := []byte(strings.Replace(string(validSampleRunSpec()), `"profiles":`, `"concurrency": {"maxConcurrency": 0}, "profiles":`, 1))
	if _, _, err := ParseAndValidateRunSpec(raw0); err == nil {
		t.Fatal("expected error on maxConcurrency=0, got nil")
	}

	// Concurrency = 4 -> error
	raw4 := []byte(strings.Replace(string(validSampleRunSpec()), `"profiles":`, `"concurrency": {"maxConcurrency": 4}, "profiles":`, 1))
	if _, _, err := ParseAndValidateRunSpec(raw4); err == nil {
		t.Fatal("expected error on maxConcurrency=4, got nil")
	}

	// Concurrency = 2 -> valid
	raw2 := []byte(strings.Replace(string(validSampleRunSpec()), `"profiles":`, `"concurrency": {"maxConcurrency": 2}, "profiles":`, 1))
	spec, _, err := ParseAndValidateRunSpec(raw2)
	if err != nil {
		t.Fatalf("unexpected error on maxConcurrency=2: %v", err)
	}
	if spec.Concurrency == nil || spec.Concurrency.MaxConcurrency != 2 {
		t.Errorf("expected maxConcurrency=2, got %+v", spec.Concurrency)
	}
}

func TestParseAndValidateRunSpec_ProfilesRequirements(t *testing.T) {
	// Empty profiles -> error
	rawNoProf := []byte(strings.Replace(string(validSampleRunSpec()), `"worker": {`, ``, 1))
	rawNoProf = []byte(strings.Replace(string(rawNoProf), `"model": "gpt-5",`, ``, 1))
	rawNoProf = []byte(strings.Replace(string(rawNoProf), `"reasoningEffort": "medium"`, ``, 1))
	rawNoProf = []byte(strings.Replace(string(rawNoProf), `}`, ``, 1))
	// Test empty profiles map
	var specMap map[string]any
	json.Unmarshal(validSampleRunSpec(), &specMap)
	specMap["profiles"] = map[string]any{}
	b, _ := json.Marshal(specMap)
	if _, _, err := ParseAndValidateRunSpec(b); err == nil {
		t.Fatal("expected error on empty profiles, got nil")
	}

	// Missing model in profile
	specMap["profiles"] = map[string]any{
		"worker": map[string]any{
			"model":           "",
			"reasoningEffort": "medium",
		},
	}
	b, _ = json.Marshal(specMap)
	if _, _, err := ParseAndValidateRunSpec(b); err == nil {
		t.Fatal("expected error on empty model in profile, got nil")
	}

	// Missing reasoningEffort in profile
	specMap["profiles"] = map[string]any{
		"worker": map[string]any{
			"model":           "gpt-5",
			"reasoningEffort": "",
		},
	}
	b, _ = json.Marshal(specMap)
	if _, _, err := ParseAndValidateRunSpec(b); err == nil {
		t.Fatal("expected error on empty reasoningEffort in profile, got nil")
	}
}

func TestParseAndValidateRunSpec_NoEndpointsOrUrls(t *testing.T) {
	for _, forbidden := range []string{`"aoUrl": "http://127.0.0.1:3000"`, `"gatewayUrl": "http://127.0.0.1:8317"`, `"endpoints": {}`} {
		raw := []byte(strings.Replace(string(validSampleRunSpec()), `"endpointPolicy"`, forbidden+`, "endpointPolicy"`, 1))
		_, _, err := ParseAndValidateRunSpec(raw)
		if err == nil {
			t.Fatalf("expected error when RunSpec contains %s, got nil", forbidden)
		}
	}
}

func TestParseAndValidateRunSpec_ToolValidation(t *testing.T) {
	// 1. Tool name with slash
	var specMap map[string]any
	json.Unmarshal(validSampleRunSpec(), &specMap)
	specMap["requiredTools"] = []map[string]any{
		{"name": "git", "required": true},
		{"name": "../evil/tool", "required": true},
	}
	badRaw, _ := json.Marshal(specMap)
	_, _, err := ParseAndValidateRunSpec(badRaw)
	if err == nil {
		t.Fatal("expected error on tool name with path separator, got nil")
	}

	// 2. Tool name with shell metacharacters
	specMap["requiredTools"] = []map[string]any{
		{"name": "git", "required": true},
		{"name": "tool;rm -rf /", "required": true},
	}
	badRaw, _ = json.Marshal(specMap)
	_, _, err = ParseAndValidateRunSpec(badRaw)
	if err == nil {
		t.Fatal("expected error on tool name with shell metacharacters, got nil")
	}

	// 3. Missing git
	specMap["requiredTools"] = []map[string]any{
		{"name": "python", "required": true},
	}
	badRaw, _ = json.Marshal(specMap)
	_, _, err = ParseAndValidateRunSpec(badRaw)
	if err == nil {
		t.Fatal("expected error when git is missing from requiredTools, got nil")
	}
}

func TestParseAndValidateRunSpec_WorkspacePolicy(t *testing.T) {
	var specMap map[string]any
	json.Unmarshal(validSampleRunSpec(), &specMap)

	// productRootMustBeClean = false
	wp := specMap["workspacePolicy"].(map[string]any)
	wp["productRootMustBeClean"] = false
	badRaw, _ := json.Marshal(specMap)
	_, _, err := ParseAndValidateRunSpec(badRaw)
	if err == nil {
		t.Fatal("expected error when productRootMustBeClean is false, got nil")
	}

	// invalid dirty policy
	wp["productRootMustBeClean"] = true
	wp["executionWorkspaceDirtyPolicy"] = "allow_any_garbage"
	badRaw, _ = json.Marshal(specMap)
	_, _, err = ParseAndValidateRunSpec(badRaw)
	if err == nil {
		t.Fatal("expected error on invalid dirty policy, got nil")
	}

	// allow_dirty_recorded is valid
	wp["executionWorkspaceDirtyPolicy"] = "allow_dirty_recorded"
	validRaw, _ := json.Marshal(specMap)
	spec, _, err := ParseAndValidateRunSpec(validRaw)
	if err != nil {
		t.Fatalf("unexpected error on allow_dirty_recorded: %v", err)
	}
	if spec.WorkspacePolicy.ExecutionWorkspaceDirtyPolicy != "allow_dirty_recorded" {
		t.Errorf("expected allow_dirty_recorded, got %s", spec.WorkspacePolicy.ExecutionWorkspaceDirtyPolicy)
	}
}
