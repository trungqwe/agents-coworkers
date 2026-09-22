# Preflight Verification Script
# Verifies environment, toolchains, git SHAs, and prerequisites

$ErrorActionPreference = "Stop"

Write-Host "=== 1. Toolchain Verification ===" -ForegroundColor Cyan
try {
    $goVer = go version
    Write-Host "[OK] Go: $goVer" -ForegroundColor Green
} catch {
    Write-Host "[FAIL] Go toolchain not found" -ForegroundColor Red
}

try {
    $codexVer = codex --version
    Write-Host "[OK] Codex CLI: $codexVer" -ForegroundColor Green
} catch {
    Write-Host "[WARN] Codex CLI not in PATH" -ForegroundColor Yellow
}

Write-Host "`n=== 2. Repository Revision Audit ===" -ForegroundColor Cyan
$repos = @(
    @{ Name = "agent-orchestrator"; Path = "D:\TU_CODE\agent-orchestrator"; Expected = "1140dd62dc7bb588b987e2c44aa1ff4796fa732b" },
    @{ Name = "CLIProxyAPI"; Path = "D:\TU_CODE\agent-orchestrator\CLIProxyAPI"; Expected = "2430354330af80b645f9ffb1a51e1e7c72c4cc8e" },
    @{ Name = "AI-Auto-Video-Creator"; Path = "D:\AI Auto Video Creator"; Expected = "4a7c8c921b7e05066505d51b168a02c3fde61317" }
)

foreach ($r in $repos) {
    if (Test-Path $r.Path) {
        $head = (git -C $r.Path rev-parse HEAD).Trim()
        $status = (git -C $r.Path status --porcelain=v2)
        $match = if ($head -eq $r.Expected) { "MATCH" } else { "DIVERGED" }
        $color = if ($match -eq "MATCH") { "Green" } else { "Yellow" }
        Write-Host "[$match] $($r.Name): $head" -ForegroundColor $color
        if ($r.Name -eq "AI-Auto-Video-Creator") {
            if ($status) {
                Write-Host "  [CRITICAL ALERT] AI-Auto-Video-Creator is NOT clean! Expected READ-ONLY!" -ForegroundColor Red
            } else {
                Write-Host "  [OK] AI-Auto-Video-Creator is clean (READ-ONLY boundary preserved)" -ForegroundColor Green
            }
        }
    } else {
        Write-Host "[MISSING] $($r.Name) not found at $($r.Path)" -ForegroundColor Red
    }
}

Write-Host "`n=== 3. Account Pool Inventory (Sanitized) ===" -ForegroundColor Cyan
$authDir = Join-Path $env:USERPROFILE ".cli-proxy-api"
if (Test-Path $authDir) {
    $codexAccs = (Get-ChildItem $authDir -Filter "codex*.json" -ErrorAction SilentlyContinue).Count
    $geminiAccs = (Get-ChildItem $authDir -Filter "antigravity*.json" -ErrorAction SilentlyContinue).Count
    Write-Host "[INFO] Codex / OpenAI usable accounts detected: $codexAccs"
    Write-Host "[INFO] Antigravity / Gemini usable accounts detected: $geminiAccs"
} else {
    Write-Host "[WARN] $authDir does not exist yet. Accounts must be logged in via cli-proxy-api." -ForegroundColor Yellow
}
