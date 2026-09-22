# Smoke test for Codex CLI through CLIProxyAPI
# Uses an isolated CODEX_HOME to guarantee user ~/.codex is never modified

param (
    [string]$ProxyUrl = "http://127.0.0.1:8317/v1",
    [string]$Model = "gpt-6-astra",
    [string]$GatewayKey = $(if ($env:CLIPROXY_KEY) { $env:CLIPROXY_KEY } else { "REDACTED_GATEWAY_KEY" }),
    [switch]$ExpectAuthBlocked = $false
)

$ErrorActionPreference = "Stop"

$proxyPort = 8317
$exePath = "D:\TU_CODE\agent-orchestrator\CLIProxyAPI\cli-proxy-api.exe"
$cfgPath = "D:\TU_CODE\agents-coworkers\config\cliproxy\config.example.yaml"
if (Test-Path "D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml") {
    $cfgPath = "D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml"
}
$proxyStartedByScript = $false
$proxyProc = $null

$tempCodexHome = "D:\TU_CODE\test-isolated-codex-$([System.Guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Path $tempCodexHome -Force | Out-Null

try {
    # Check if proxy is already listening
    $tcp = New-Object System.Net.Sockets.TcpClient
    $listening = $false
    try {
        $tcp.Connect("127.0.0.1", $proxyPort)
        $listening = $true
    } catch {
        $listening = $false
    } finally {
        $tcp.Close()
    }

    if (-not $listening) {
        Write-Host "Starting CLIProxyAPI on port $proxyPort..." -ForegroundColor Cyan
        $proxyProc = Start-Process -FilePath $exePath -ArgumentList "-config", "`"$cfgPath`"" -PassThru -WindowStyle Hidden
        $proxyStartedByScript = $true
        Start-Sleep -Seconds 2
    }

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
    [System.IO.File]::WriteAllText((Join-Path $tempCodexHome "config.toml"), $configContent, [System.Text.UTF8Encoding]::new($false))

    $env:CODEX_HOME = $tempCodexHome
    $env:CLIPROXY_KEY = $GatewayKey

    Write-Host "Testing codex exec with model: $Model through $ProxyUrl" -ForegroundColor Cyan
    
    $oldEAP = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $output = cmd /c "codex exec --ephemeral --skip-git-repo-check --model $Model "Respond with: CODEX_CLIPROXY_INTEGRATION_OK" < nul 2>&1"
    $codexExit = $LASTEXITCODE
    $ErrorActionPreference = $oldEAP
    $outputStr = ($output -join "`n")

    Write-Host "Codex exit code: $codexExit"
    Write-Host "Codex output:`n$outputStr"

    if ($ExpectAuthBlocked) {
        $expectedSignatures = @(
            "model_not_found",
            "unknown provider for model"
        )
        $foundExpectedSignature = $false
        foreach ($sig in $expectedSignatures) {
            if ($outputStr -match [regex]::Escape($sig)) {
                $foundExpectedSignature = $true
                break
            }
        }

        if ($codexExit -ne 0 -and $foundExpectedSignature) {
            Write-Host "[PRE-AUTH EXPECTED] Codex command failed with code $codexExit matching documented unauthenticated signature ('model_not_found' / 'unknown provider for model')." -ForegroundColor Green
            return
        } else {
            Write-Error "FAIL-CLOSED: -ExpectAuthBlocked requires specific unauthenticated gateway signature ('model_not_found' / 'unknown provider for model'). Arbitrary failures, syntax errors, or connection drops remain FAIL. Code: $codexExit, Output: $outputStr"
            exit 1
        }
    }

    # Strict fail-closed verification (default behavior)
    if ($codexExit -ne 0) {
        Write-Error "FAIL-CLOSED: Codex command returned exit code ${codexExit}. Output: $outputStr"
        exit 1
    }

    if ($outputStr -notmatch "CODEX_CLIPROXY_INTEGRATION_OK") {
        Write-Error "FAIL-CLOSED: Expected marker 'CODEX_CLIPROXY_INTEGRATION_OK' missing from output: $outputStr"
        exit 1
    }

    Write-Host "[PASS] Codex integration verified through CLIProxyAPI." -ForegroundColor Green
} finally {
    Write-Host "Cleaning up test CODEX_HOME..." -ForegroundColor Cyan
    Remove-Item -Path $tempCodexHome -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item Env:CODEX_HOME -ErrorAction SilentlyContinue
    Remove-Item Env:CLIPROXY_KEY -ErrorAction SilentlyContinue

    if ($proxyStartedByScript -and $proxyProc) {
        Write-Host "Stopping script-managed CLIProxyAPI..." -ForegroundColor Cyan
        Stop-Process -Id $proxyProc.Id -Force -ErrorAction SilentlyContinue
    }
}
