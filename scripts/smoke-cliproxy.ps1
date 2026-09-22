# Smoke test for CLIProxyAPI
# Starts proxy locally, tests /v1/models catalog, and shuts down

$ErrorActionPreference = "Stop"

$port = 8317
$exePath = "D:\TU_CODE\agent-orchestrator\CLIProxyAPI\cli-proxy-api.exe"
$cfgPath = "D:\TU_CODE\agents-coworkers\config\cliproxy\config.example.yaml"

Write-Host "=== Starting CLIProxyAPI smoke test ===" -ForegroundColor Cyan

# Start process
$proc = Start-Process -FilePath $exePath -ArgumentList "-config", "`"$cfgPath`"" -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 2

try {
    Write-Host "Querying /v1/models..."
    $resp = Invoke-RestMethod -Uri "http://127.0.0.1:$port/v1/models" -Method Get -Headers @{ "Authorization" = "Bearer REDACTED_GATEWAY_KEY" }
    $modelIds = $resp.data | ForEach-Object { $_.id }
    
    $hasAstra = $modelIds -contains "gpt-6-astra"
    $hasGemini = $modelIds -contains "gemini-3.8-flash-high"
    
    Write-Host "Catalog contains gpt-6-astra: $hasAstra"
    Write-Host "Catalog contains gemini-3.8-flash-high: $hasGemini"
    
    if (-not $hasAstra -or -not $hasGemini) {
        Write-Host "Available models sample: $($modelIds[0..10] -join ', ')"
    }
} finally {
    Write-Host "Stopping CLIProxyAPI..."
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
}
