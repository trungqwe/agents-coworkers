# Preflight Verification Script
# Verifies environment, toolchains, git SHAs, working tree boundaries, and auth inventory.
# Fails closed ($preflightFailed accumulator, exit 1) if any critical prerequisite fails.

$ErrorActionPreference = "Stop"
$preflightFailed = $false

Write-Host "=== 1. Toolchain & Runtime Prerequisites ===" -ForegroundColor Cyan

# 1.1 PowerShell
try {
    $psVer = $PSVersionTable.PSVersion
    Write-Host "[OK] PowerShell: $psVer" -ForegroundColor Green
} catch {
    Write-Host "[FAIL] PowerShell version check failed" -ForegroundColor Red
    $preflightFailed = $true
}

# 1.2 Git
try {
    $gitVer = git --version
    Write-Host "[OK] Git: $gitVer" -ForegroundColor Green
} catch {
    Write-Host "[FAIL] Git not found in PATH" -ForegroundColor Red
    $preflightFailed = $true
}

# 1.3 Go
try {
    $goVer = go version
    Write-Host "[OK] Go: $goVer" -ForegroundColor Green
} catch {
    Write-Host "[FAIL] Go toolchain not found in PATH" -ForegroundColor Red
    $preflightFailed = $true
}

# 1.4 Codex CLI
try {
    $codexVer = codex --version
    Write-Host "[OK] Codex CLI: $codexVer" -ForegroundColor Green
} catch {
    Write-Host "[FAIL] Codex CLI not found in PATH" -ForegroundColor Red
    $preflightFailed = $true
}

# 1.5 Node.js
try {
    $nodeVer = node --version
    Write-Host "[OK] Node.js: $nodeVer" -ForegroundColor Green
} catch {
    Write-Host "[FAIL] Node.js not found in PATH" -ForegroundColor Red
    $preflightFailed = $true
}

# 1.6 npm
try {
    $npmVer = npm --version
    Write-Host "[OK] npm: $npmVer" -ForegroundColor Green
} catch {
    Write-Host "[FAIL] npm not found in PATH" -ForegroundColor Red
    $preflightFailed = $true
}

Write-Host "`n=== 2. Repository Revision & Boundary Audit ===" -ForegroundColor Cyan
$repos = @(
    @{ Name = "agent-orchestrator"; Path = "D:\TU_CODE\agent-orchestrator"; Expected = "1140dd62dc7bb588b987e2c44aa1ff4796fa732b" },
    @{ Name = "CLIProxyAPI"; Path = "D:\TU_CODE\agent-orchestrator\CLIProxyAPI"; Expected = "2430354330af80b645f9ffb1a51e1e7c72c4cc8e" },
    @{ Name = "AI-Auto-Video-Creator"; Path = "D:\AI Auto Video Creator"; Expected = "4a7c8c921b7e05066505d51b168a02c3fde61317" }
)

foreach ($r in $repos) {
    if (-not (Test-Path $r.Path)) {
        Write-Host "[MISSING] $($r.Name) not found at $($r.Path)" -ForegroundColor Red
        $preflightFailed = $true
        continue
    }

    try {
        $head = (git -C $r.Path rev-parse HEAD).Trim()
        if ($head -ne $r.Expected) {
            Write-Host "[DIVERGED] $($r.Name): $head (Expected: $($r.Expected))" -ForegroundColor Red
            $preflightFailed = $true
        } else {
            Write-Host "[MATCH] $($r.Name): $head" -ForegroundColor Green
        }
    } catch {
        Write-Host "[FAIL] Unable to determine git revision for $($r.Name)" -ForegroundColor Red
        $preflightFailed = $true
        continue
    }

    # Repository boundary & cleanliness checks
    if ($r.Name -eq "AI-Auto-Video-Creator") {
        # Strictly READ-ONLY (INV-001): absolutely zero changes permitted
        $prodStatus = (git -C $r.Path status --porcelain)
        if ($prodStatus) {
            Write-Host "  [CRITICAL ALERT] AI-Auto-Video-Creator is NOT clean! Expected READ-ONLY (INV-001)!" -ForegroundColor Red
            $preflightFailed = $true
        } else {
            Write-Host "  [OK] AI-Auto-Video-Creator is clean (READ-ONLY boundary preserved)" -ForegroundColor Green
        }
    } elseif ($r.Name -eq "agent-orchestrator") {
        # Upstream clean by default (INV-002): tracked files must be clean; untracked only allows nested CLIProxyAPI
        $aoTrackedDiff = (git -C $r.Path status --porcelain -uno)
        if ($aoTrackedDiff) {
            Write-Host "  [FAIL] agent-orchestrator has uncommitted tracked changes! Expected clean upstream (INV-002)!" -ForegroundColor Red
            $preflightFailed = $true
        } else {
            Write-Host "  [OK] agent-orchestrator upstream working tree is clean" -ForegroundColor Green
        }
    } elseif ($r.Name -eq "CLIProxyAPI") {
        # CLIProxyAPI: tracked files must be clean
        $cliproxyTrackedDiff = (git -C $r.Path status --porcelain -uno)
        if ($cliproxyTrackedDiff) {
            Write-Host "  [FAIL] CLIProxyAPI has uncommitted tracked changes!" -ForegroundColor Red
            $preflightFailed = $true
        } else {
            Write-Host "  [OK] CLIProxyAPI working tree is clean" -ForegroundColor Green
        }
    }
}

Write-Host "`n=== 3. Account Pool Inventory (Canonical Delegation) ===" -ForegroundColor Cyan
$authInventory = Join-Path $PSScriptRoot "auth-inventory.ps1"
if (-not (Test-Path $authInventory)) {
    Write-Host "[FAIL] Sanitized auth inventory script missing: $authInventory" -ForegroundColor Red
    $preflightFailed = $true
} else {
    try {
        & $authInventory
    } catch {
        Write-Host "[FAIL] Auth inventory execution failed: $_" -ForegroundColor Red
        $preflightFailed = $true
    }
}

Write-Host ""
if ($preflightFailed) {
    Write-Error "Preflight failed."
    exit 1
}

Write-Host "[PASS] Preflight verification complete." -ForegroundColor Green
exit 0
