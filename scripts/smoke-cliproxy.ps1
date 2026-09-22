# Smoke test for CLIProxyAPI
# Reuses existing gateway if already listening on port 8317, or starts and manages one if not.
# Queries /v1/models catalog and enforces fail-closed model verification.

param (
    [string]$GatewayKey = $(if ($env:CLIPROXY_KEY) { $env:CLIPROXY_KEY } else { "REDACTED_GATEWAY_KEY" }),
    [switch]$AllowUnauthenticated = $false
)

$ErrorActionPreference = "Stop"

$port = 8317
$exePath = "D:\TU_CODE\agent-orchestrator\CLIProxyAPI\cli-proxy-api.exe"
$cfgPath = "D:\TU_CODE\agents-coworkers\config\cliproxy\config.example.yaml"
if (Test-Path "D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml") {
    $cfgPath = "D:\TU_CODE\agents-coworkers\config\cliproxy\config.runtime.yaml"
}

$proxyStartedByScript = $false
$proxyProc = $null

Write-Host "=== Starting CLIProxyAPI smoke test ===" -ForegroundColor Cyan

try {
    # Probe 127.0.0.1:$port to check if gateway is already listening
    $tcp = New-Object System.Net.Sockets.TcpClient
    $listening = $false
    try {
        $tcp.Connect("127.0.0.1", $port)
        $listening = $true
    } catch {
        $listening = $false
    } finally {
        $tcp.Close()
    }

    if ($listening) {
        Write-Host "Reusing existing CLIProxyAPI gateway listening on 127.0.0.1:$port (external process preserved)." -ForegroundColor Cyan
    } else {
        Write-Host "Starting CLIProxyAPI on port $port with config $cfgPath..." -ForegroundColor Cyan
        $proxyProc = Start-Process -FilePath $exePath -ArgumentList "-config", "`"$cfgPath`"" -PassThru -WindowStyle Hidden
        $proxyStartedByScript = $true

        # Probe until ready with a timeout
        $maxWaitSeconds = 10
        $ready = $false
        $sw = [System.Diagnostics.Stopwatch]::StartNew()
        while ($sw.Elapsed.TotalSeconds -lt $maxWaitSeconds) {
            Start-Sleep -Milliseconds 300
            $probeTcp = New-Object System.Net.Sockets.TcpClient
            try {
                $probeTcp.Connect("127.0.0.1", $port)
                $ready = $true
                break
            } catch {
                # continue waiting
            } finally {
                $probeTcp.Close()
            }
        }

        if (-not $ready) {
            Write-Error "FAIL: CLIProxyAPI failed to start and become reachable on 127.0.0.1:$port within $maxWaitSeconds seconds."
            exit 1
        }
        Write-Host "CLIProxyAPI gateway reachable on 127.0.0.1:$port." -ForegroundColor Green
    }

    Write-Host "Querying /v1/models on 127.0.0.1:$port..."
    $resp = Invoke-RestMethod -Uri "http://127.0.0.1:$port/v1/models" -Method Get -Headers @{ "Authorization" = "Bearer $GatewayKey" }
    $modelIds = @($resp.data | ForEach-Object { $_.id })
    
    $hasAstra = $modelIds -contains "gpt-6-astra"
    $hasGemini = $modelIds -contains "gemini-3.8-flash-high"
    
    Write-Host "Catalog total models: $($modelIds.Count)"
    Write-Host "Catalog contains gpt-6-astra: $hasAstra"
    Write-Host "Catalog contains gemini-3.8-flash-high: $hasGemini"
    
    if (-not $hasAstra -or -not $hasGemini) {
        Write-Warning "[BLOCKED_RUNTIME_AUTH] Required target models are absent from the active catalog. Check auth inventory, model registration, and provider routing."
        if (-not $AllowUnauthenticated) {
            Write-Error "FAIL-CLOSED: Required target models are missing from the active catalog (gpt-6-astra=$hasAstra, gemini-3.8-flash-high=$hasGemini). Check auth inventory, provider registration, model catalog, and routing."
            exit 1
        }
    } else {
        Write-Host "[PASS] Both target models (gpt-6-astra, gemini-3.8-flash-high) present in active catalog." -ForegroundColor Green
    }
} finally {
    if ($proxyStartedByScript -and $proxyProc) {
        Write-Host "Stopping script-managed CLIProxyAPI (PID $($proxyProc.Id))..." -ForegroundColor Cyan
        Stop-Process -Id $proxyProc.Id -Force -ErrorAction SilentlyContinue
    } else {
        Write-Host "Preserving external CLIProxyAPI instance." -ForegroundColor Gray
    }
}