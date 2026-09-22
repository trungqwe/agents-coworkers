# Sanitized Auth Inventory for CLIProxyAPI
# Scans ~/.cli-proxy-api for credentials and reports counts and sanitized IDs.
# NEVER prints tokens, keys, passwords, raw filenames, emails, or raw secrets.

param (
    [string]$AuthDir = $(
        if ($env:CLIPROXY_AUTH_DIR) { $env:CLIPROXY_AUTH_DIR }
        elseif ($env:USERPROFILE) { Join-Path $env:USERPROFILE ".cli-proxy-api" }
        elseif ($HOME) { Join-Path $HOME ".cli-proxy-api" }
        else { "~/.cli-proxy-api" }
    )
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

foreach ($file in $authFiles) {
    # Generate stable truncated hash from filename without exposing identity/email preimage
    $sha256 = [System.Security.Cryptography.SHA256]::Create()
    $hashBytes = $sha256.ComputeHash([System.Text.Encoding]::UTF8.GetBytes($file.Name))
    $credentialHash = [System.BitConverter]::ToString($hashBytes).Replace("-", "").Substring(0, 16).ToLower()

    $providerType = "unknown"

    try {
        # Determine provider type safely without exposing file contents or logging
        if ($file.Name -match "^codex-") {
            $providerType = "codex"
        } elseif ($file.Name -match "^antigravity-") {
            $providerType = "antigravity"
        } else {
            $json = Get-Content -Path $file.FullName -Raw | ConvertFrom-Json -ErrorAction SilentlyContinue
            if ($json -and $json.type) {
                $providerType = [string]$json.type
            }
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
        ProviderType   = $providerType
        CredentialHash = $credentialHash
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
    Write-Host "`nCredential Inventory (Sanitized - No PII/Preimages):" -ForegroundColor Yellow
    $inventory | Format-Table ProviderType, CredentialHash, LastModified -AutoSize
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