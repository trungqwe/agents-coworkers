# Redact Evidence Script
# Scans files in evidence/ for sensitive secrets, tokens, emails, and paths
# Replaces matching patterns with [REDACTED_*] placeholders

param (
    [string]$EvidenceDir = "D:\TU_CODE\agents-coworkers\evidence"
)

$ErrorActionPreference = "Stop"

Write-Host "=== Scanning evidence files for sensitive tokens ===" -ForegroundColor Cyan

$sensitivePatterns = @(
    @{ Pattern = '(?i)(bearer\s+)[A-Za-z0-9\-_\.]{20,}'; Replacement = '$1[REDACTED_BEARER_TOKEN]' },
    @{ Pattern = '(?i)(access_token["\s:=]+)[A-Za-z0-9\-_\.]{20,}'; Replacement = '$1[REDACTED_ACCESS_TOKEN]' },
    @{ Pattern = '(?i)(refresh_token["\s:=]+)[A-Za-z0-9\-_\.]{20,}'; Replacement = '$1[REDACTED_REFRESH_TOKEN]' },
    @{ Pattern = '(?i)(id_token["\s:=]+)[A-Za-z0-9\-_\.]{20,}'; Replacement = '$1[REDACTED_ID_TOKEN]' },
    @{ Pattern = '(?i)(api[_-]?key["\s:=]+)[A-Za-z0-9\-_\.]{16,}'; Replacement = '$1[REDACTED_API_KEY]' },
    @{ Pattern = '(?i)(client_secret["\s:=]+)[A-Za-z0-9\-_\.]{16,}'; Replacement = '$1[REDACTED_CLIENT_SECRET]' },
    @{ Pattern = '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}'; Replacement = '[REDACTED_EMAIL]' },
    @{ Pattern = '(?i)C:\\Users\\[A-Za-z0-9._-]+\\'; Replacement = 'C:\Users\[REDACTED_USER]\\' }
)

$files = Get-ChildItem -Path $EvidenceDir -Recurse -File -ErrorAction SilentlyContinue

$clean = $true
foreach ($file in $files) {
    $content = Get-Content -Path $file.FullName -Raw -Encoding UTF8
    $modified = $content
    foreach ($sp in $sensitivePatterns) {
        if ($modified -match $sp.Pattern) {
            $clean = $false
            Write-Host "Redacting sensitive pattern in: $($file.Name)" -ForegroundColor Yellow
            $modified = [regex]::Replace($modified, $sp.Pattern, $sp.Replacement)
        }
    }
    if ($modified -ne $content) {
        [System.IO.File]::WriteAllText($file.FullName, $modified, [System.Text.UTF8Encoding]::new($false))
    }
}

if ($clean) {
    Write-Host "[OK] All evidence files clean. No sensitive credentials found." -ForegroundColor Green
} else {
    Write-Host "[INFO] Redaction completed on matching files." -ForegroundColor Green
}
