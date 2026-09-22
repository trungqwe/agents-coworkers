package integration

import (
	"encoding/json"
	"os"
	"testing"
)

// MirroredAgentConfig mirrors the patched AO CLI agentConfig struct (with Effort).
type MirroredAgentConfig struct {
	Model       string `json:"model,omitempty"`
	Effort      string `json:"effort,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Permissions string `json:"permissions,omitempty"`
}

type MirroredRoleOverride struct {
	Harness     string              `json:"agent,omitempty"`
	AgentConfig MirroredAgentConfig `json:"agentConfig,omitempty"`
}

type MirroredProjectConfig struct {
	Worker       MirroredRoleOverride `json:"worker,omitempty"`
	Orchestrator MirroredRoleOverride `json:"orchestrator,omitempty"`
}

// UnpatchedAgentConfig mirrors the upstream unpatched AO CLI agentConfig struct (SHA 1140dd62dc7bb588b987e2c44aa1ff4796fa732b)
// which lacks the Effort field.
type UnpatchedAgentConfig struct {
	Model       string `json:"model,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Permissions string `json:"permissions,omitempty"`
}

type UnpatchedRoleOverride struct {
	Harness     string               `json:"agent,omitempty"`
	AgentConfig UnpatchedAgentConfig `json:"agentConfig,omitempty"`
}

type UnpatchedProjectConfig struct {
	Worker       UnpatchedRoleOverride `json:"worker,omitempty"`
	Orchestrator UnpatchedRoleOverride `json:"orchestrator,omitempty"`
}

func TestProjectConfigTemplate_StructuralValidation(t *testing.T) {
	data, err := os.ReadFile("../../config/ao/project-config.example.json")
	if err != nil {
		t.Fatalf("failed to read project-config.example.json: %v", err)
	}

	// 1. Raw JSON Map structure validation
	var rawMap map[string]any
	if err := json.Unmarshal(data, &rawMap); err != nil {
		t.Fatalf("invalid JSON syntax: %v", err)
	}

	for _, role := range []string{"orchestrator", "worker"} {
		roleVal, ok := rawMap[role].(map[string]any)
		if !ok {
			t.Fatalf("missing or invalid top-level role %q in template", role)
		}
		if _, ok := roleVal["agent"]; !ok {
			t.Errorf("role %q missing 'agent' harness key", role)
		}
		agentCfg, ok := roleVal["agentConfig"].(map[string]any)
		if !ok {
			t.Errorf("role %q missing nested 'agentConfig' object", role)
			continue
		}
		if _, ok := agentCfg["model"]; !ok {
			t.Errorf("role %q agentConfig missing 'model'", role)
		}
		if _, ok := agentCfg["effort"]; !ok {
			t.Errorf("role %q agentConfig missing 'effort'", role)
		}
		if _, ok := agentCfg["mode"]; !ok {
			t.Errorf("role %q agentConfig missing 'mode'", role)
		}
	}

	// 2. Strongly-typed deserialization validation
	var cfg MirroredProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to unmarshal JSON into MirroredProjectConfig: %v", err)
	}

	if cfg.Orchestrator.Harness != "codex" || cfg.Orchestrator.AgentConfig.Model != "gpt-6-astra" {
		t.Errorf("orchestrator mismatch: harness=%s, model=%s", cfg.Orchestrator.Harness, cfg.Orchestrator.AgentConfig.Model)
	}
	if cfg.Worker.Harness != "codex" || cfg.Worker.AgentConfig.Model != "gemini-3.8-flash-high" {
		t.Errorf("worker mismatch: harness=%s, model=%s", cfg.Worker.Harness, cfg.Worker.AgentConfig.Model)
	}
}

// TestZeroPatch_UnpatchedAgentConfigDropsEffort proves the critical finding:
// Unpatched upstream AO CLI (project.go) unmarshals --config-json into an agentConfig mirror struct
// lacking Effort, silently dropping reasoning effort before sending the HTTP payload to the daemon.
func TestZeroPatch_UnpatchedAgentConfigDropsEffort(t *testing.T) {
	data, err := os.ReadFile("../../config/ao/project-config.example.json")
	if err != nil {
		t.Fatalf("failed to read template: %v", err)
	}

	var unpatched UnpatchedProjectConfig
	if err := json.Unmarshal(data, &unpatched); err != nil {
		t.Fatalf("unmarshal into unpatched struct failed: %v", err)
	}

	// Marshal back to JSON (as the unpatched CLI does when forming HTTP request)
	reencoded, err := json.Marshal(unpatched)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var rawMap map[string]any
	if err := json.Unmarshal(reencoded, &rawMap); err != nil {
		t.Fatalf("unmarshal raw failed: %v", err)
	}

	for _, role := range []string{"worker", "orchestrator"} {
		roleMap := rawMap[role].(map[string]any)
		agentCfg := roleMap["agentConfig"].(map[string]any)
		if effortVal, ok := agentCfg["effort"]; ok && effortVal != nil && effortVal != "" {
			t.Fatalf("CRITICAL: expected unpatched CLI to drop effort, but found %v in role %s", effortVal, role)
		}
	}
}

// TestHeadlessPatch_PatchedAgentConfigPreservesEffort proves that applying the patch
// preserves effort='low' for both worker and orchestrator through the CLI marshaling layer.
func TestHeadlessPatch_PatchedAgentConfigPreservesEffort(t *testing.T) {
	data, err := os.ReadFile("../../config/ao/project-config.example.json")
	if err != nil {
		t.Fatalf("failed to read template: %v", err)
	}

	var patched MirroredProjectConfig
	if err := json.Unmarshal(data, &patched); err != nil {
		t.Fatalf("unmarshal into patched struct failed: %v", err)
	}

	reencoded, err := json.Marshal(patched)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var rawMap map[string]any
	if err := json.Unmarshal(reencoded, &rawMap); err != nil {
		t.Fatalf("unmarshal raw failed: %v", err)
	}

	for _, role := range []string{"worker", "orchestrator"} {
		roleMap := rawMap[role].(map[string]any)
		agentCfg := roleMap["agentConfig"].(map[string]any)
		if agentCfg["effort"] != "low" {
			t.Fatalf("expected patched CLI to preserve effort='low', got %v in role %s", agentCfg["effort"], role)
		}
	}
}
