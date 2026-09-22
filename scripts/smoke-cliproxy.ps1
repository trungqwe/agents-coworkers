# Smoke test for CLIProxyAPI
# Starts proxy locally, queries /v1/models catalog, and enforces fail-closed model verification

param (
    [switch]$AllowUnauthenticated = $false
)

$ErrorActionPreference = "Stop"

$port = 8317
$exePath = "D:\TU_CODE\agent-orchestrator\CLIProxyAPI\cli-proxy-api.exe"
$cfgPath = "D:\TU_CODE\agents-coworkers\config\cliproxy\config.example.yaml"

Write-Host "=== Starting CLIProxyAPI smoke test ===" -ForegroundColor Cyan

# Start process
$proc = Start-Process -FilePath $exePath -ArgumentList "-config", "`"$cfgPath`"" -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 2

try {
    Write-Host "Querying /v1/models on 127.0.0.1:$port..."
    $resp = Invoke-RestMethod -Uri "http://127.0.0.1:$port/v1/models" -Method Get -Headers @{ "Authorization" = "Bearer REDACTED_GATEWAY_KEY" }
    $modelIds = @($resp.data | ForEach-Object { $_.id })
    
    $hasAstra = $modelIds -contains "gpt-6-astra"
    $hasGemini = $modelIds -contains "gemini-3.8-flash-high"
    
    Write-Host "Catalog total models: $($modelIds.Count)"
    Write-Host "Catalog contains gpt-6-astra: $hasAstra"
    Write-Host "Catalog contains gemini-3.8-flash-high: $hasGemini"
    
    if (-not $hasAstra -or -not $hasGemini) {
        Write-Warning "[BLOCKED_RUNTIME_AUTH] Required models absent from active catalog because 0 accounts are authenticated in ~/.cli-proxy-api."
        if (-not $AllowUnauthenticated) {
            Write-Error "FAIL-CLOSED: Target models missing from catalog (gpt-6-astra=$hasAstra, gemini-3.8-flash-high=$hasGemini). Complete OAuth login first."
            exit 1
        }
    } else {
        Write-Host "[PASS] Both target models (gpt-6-astra, gemini-3.8-flash-high) present in active catalog." -ForegroundColor Green
    }
} finally {
    Write-Host "Stopping CLIProxyAPI..."
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
}
