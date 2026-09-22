# Smoke test for Codex CLI through CLIProxyAPI
# Uses an isolated CODEX_HOME to guarantee user ~/.codex is never modified

param (
    [string]$ProxyUrl = "http://127.0.0.1:8317/v1",
    [string]$Model = "gpt-6-astra"
)

$ErrorActionPreference = "Stop"

$tempCodexHome = Join-Path ([System.IO.Path]::GetTempPath()) "codex_cliproxy_test_$([System.Guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Path $tempCodexHome -Force | Out-Null

try {
    Write-Host "Setting up isolated CODEX_HOME: $tempCodexHome" -ForegroundColor Cyan
    
    $configContent = @"
model_provider = "cliproxy"
model = "$Model"
model_reasoning_effort = "low"

[model_providers.cliproxy]
name = "CLIProxyAPI Gateway"
base_url = "$ProxyUrl"
wire_api = "responses"
env_key = "CLIPROXY_KEY"

[features]
multi_agent = false
"@
    Set-Content -Path (Join-Path $tempCodexHome "config.toml") -Value $configContent -Encoding UTF8

    $env:CODEX_HOME = $tempCodexHome
    $env:CLIPROXY_KEY = "REDACTED_GATEWAY_KEY"

    Write-Host "Testing codex exec with model: $Model through $ProxyUrl" -ForegroundColor Cyan
    # Run codex exec non-interactively
    $output = codex exec --model $Model "Respond with: CODEX_CLIPROXY_INTEGRATION_OK" 2>&1
    Write-Host "Output: $output"
} finally {
    Write-Host "Cleaning up test CODEX_HOME..." -ForegroundColor Cyan
    Remove-Item -Path $tempCodexHome -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item Env:CODEX_HOME -ErrorAction SilentlyContinue
    Remove-Item Env:CLIPROXY_KEY -ErrorAction SilentlyContinue
}
