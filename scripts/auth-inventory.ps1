# Sanitized Auth Inventory for CLIProxyAPI
# Scans ~/.cli-proxy-api for credentials and reports counts and sanitized IDs
# NEVER prints tokens, keys, passwords, or raw secrets.

param (
    [string]$AuthDir = "C:\Users\Admin\.cli-proxy-api"
)

$ErrorActionPreference = "Stop"

Write-Host "=== CLIProxyAPI Sanitized Auth Inventory ===" -ForegroundColor Cyan
Write-Host "Auth Directory: $AuthDir"

if (-not (Test-Path $AuthDir)) {
    Write-Warning "Auth directory does not exist: $AuthDir"
    exit 0
}

# Scan .json files in $AuthDir (excluding subdirectories like logs)
$authFiles = Get-ChildItem -Path $AuthDir -File -Filter "*.json" -ErrorAction SilentlyContinue

$codexCount = 0
$antigravityCount = 0
$otherCount = 0

$inventory = @()
$seenNames = @{}

foreach ($file in $authFiles) {
    $fname = $file.Name
    $isDuplicate = $seenNames.ContainsKey($fname)
    $seenNames[$fname] = $true

    $providerType = "unknown"
    $sanitizedId = [System.BitConverter]::ToString(([System.Security.Cryptography.SHA256]::Create().ComputeHash([System.Text.Encoding]::UTF8.GetBytes($fname)))).Replace("-", "").Substring(0, 12).ToLower()

    try {
        # Read only top-level metadata keys to detect provider type WITHOUT logging tokens
        $json = Get-Content -Path $file.FullName -Raw | ConvertFrom-Json -ErrorAction SilentlyContinue
        if ($json.type) {
            $providerType = [string]$json.type
        } elseif ($fname -match "codex") {
            $providerType = "codex"
        } elseif ($fname -match "antigravity" -or $fname -match "gemini") {
            $providerType = "antigravity"
        }
    } catch {
        $providerType = "unparseable"
    }

    if ($providerType -match "codex") {
        $codexCount++
    } elseif ($providerType -match "antigravity") {
        $antigravityCount++
    } else {
        $otherCount++
    }

    $inventory += [PSCustomObject]@{
        Filename       = $fname
        ProviderType   = $providerType
        IdentifierHash = $sanitizedId
        Duplicate      = $isDuplicate
        LastModified   = $file.LastWriteTime.ToString("yyyy-MM-dd HH:mm:ss")
    }
}

Write-Host "`nCredential Counts:" -ForegroundColor Yellow
Write-Host "  Codex (ChatGPT Plus) : $codexCount (Target: 6)"
Write-Host "  Antigravity (Gemini) : $antigravityCount (Target: 8)"
if ($otherCount -gt 0) {
    Write-Host "  Other Providers      : $otherCount"
}

if ($inventory.Count -gt 0) {
    Write-Host "`nCredential Inventory (Sanitized):" -ForegroundColor Yellow
    $inventory | Format-Table Filename, ProviderType, IdentifierHash, Duplicate, LastModified -AutoSize
} else {
    Write-Host "`nNo credential files currently found in $AuthDir." -ForegroundColor Gray
}

Write-Host "`nStatus:" -ForegroundColor Cyan
if ($codexCount -eq 0 -and $antigravityCount -eq 0) {
    Write-Host "  [PENDING_AUTH] 0 accounts authenticated. Run -codex-login and -antigravity-login." -ForegroundColor Yellow
} elseif ($codexCount -ge 6 -and $antigravityCount -ge 8) {
    Write-Host "  [AUTH_TARGET_MET] Targets reached (Codex: $codexCount/6, Antigravity: $antigravityCount/8)." -ForegroundColor Green
} else {
    Write-Host "  [PARTIAL_AUTH] In-progress: Codex $codexCount/6, Antigravity $antigravityCount/8." -ForegroundColor Yellow
}
