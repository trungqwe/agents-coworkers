package integration

import (
	"encoding/json"
	"os"
	"testing"
)

// MirroredRoleOverride replicates the wire contract of AO's domain.RoleOverride for static template validation.
// NOTE: This test validates the structural format of the example configuration template against the documented
// schema; full daemon REST API and CLI roundtrips are validated within the Agent Orchestrator test suite.
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
