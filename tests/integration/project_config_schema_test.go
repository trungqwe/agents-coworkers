package integration

import (
	"encoding/json"
	"os"
	"testing"
)

type AgentConfig struct {
	Model       string `json:"model,omitempty"`
	Effort      string `json:"effort,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Permissions string `json:"permissions,omitempty"`
}

type RoleOverride struct {
	Harness     string      `json:"agent,omitempty"`
	AgentConfig AgentConfig `json:"agentConfig,omitempty"`
}

type ProjectConfig struct {
	Worker       RoleOverride `json:"worker,omitempty"`
	Orchestrator RoleOverride `json:"orchestrator,omitempty"`
}

func TestProjectConfigSchema_RoleOverrideDeserialization(t *testing.T) {
	data, err := os.ReadFile("../../config/ao/project-config.example.json")
	if err != nil {
		t.Fatalf("failed to read project-config.example.json: %v", err)
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to unmarshal JSON into ProjectConfig: %v", err)
	}

	// Verify Orchestrator
	if cfg.Orchestrator.Harness != "codex" {
		t.Errorf("expected orchestrator.agent=codex, got %q", cfg.Orchestrator.Harness)
	}
	if cfg.Orchestrator.AgentConfig.Model != "gpt-6-astra" {
		t.Errorf("expected orchestrator.agentConfig.model=gpt-6-astra, got %q", cfg.Orchestrator.AgentConfig.Model)
	}
	if cfg.Orchestrator.AgentConfig.Effort != "low" {
		t.Errorf("expected orchestrator.agentConfig.effort=low, got %q", cfg.Orchestrator.AgentConfig.Effort)
	}
	if cfg.Orchestrator.AgentConfig.Mode != "chat" {
		t.Errorf("expected orchestrator.agentConfig.mode=chat, got %q", cfg.Orchestrator.AgentConfig.Mode)
	}

	// Verify Worker
	if cfg.Worker.Harness != "codex" {
		t.Errorf("expected worker.agent=codex, got %q", cfg.Worker.Harness)
	}
	if cfg.Worker.AgentConfig.Model != "gemini-3.8-flash-high" {
		t.Errorf("expected worker.agentConfig.model=gemini-3.8-flash-high, got %q", cfg.Worker.AgentConfig.Model)
	}
	if cfg.Worker.AgentConfig.Effort != "low" {
		t.Errorf("expected worker.agentConfig.effort=low, got %q", cfg.Worker.AgentConfig.Effort)
	}
	if cfg.Worker.AgentConfig.Mode != "chat" {
		t.Errorf("expected worker.agentConfig.mode=chat, got %q", cfg.Worker.AgentConfig.Mode)
	}
}
