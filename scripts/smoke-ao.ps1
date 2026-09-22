# Smoke test for Git Worktree Primitive Isolation
# Tests Git worktree creation, branch isolation, and multi-worker commit primitives (GIT_WORKTREE_PRIMITIVE_PROVEN)
# NOTE: This validates local Git worktree mechanics as used by AO, NOT live AO daemon sessions with LLM agents.

param (
    [string]$RepoPath = "D:\TU_CODE\test-disposable-repo",
    [string]$WorktreeBase = "D:\TU_CODE\test-disposable-worktrees",
    [int]$WorkerCount = 3
)

$ErrorActionPreference = "Stop"

Write-Host "=== 1. Setting up disposable Git repository for AO worktree primitive verification ===" -ForegroundColor Cyan
if (Test-Path $RepoPath) { Remove-Item -Path $RepoPath -Recurse -Force -ErrorAction SilentlyContinue }
if (Test-Path $WorktreeBase) { Remove-Item -Path $WorktreeBase -Recurse -Force -ErrorAction SilentlyContinue }

New-Item -ItemType Directory -Path $RepoPath -Force | Out-Null
New-Item -ItemType Directory -Path $WorktreeBase -Force | Out-Null

git -C $RepoPath init
"Initial project baseline" | Set-Content (Join-Path $RepoPath "README.md")
git -C $RepoPath add README.md
git -C $RepoPath commit -m "chore: initial commit"

Write-Host "Repository initialized at $RepoPath" -ForegroundColor Green

Write-Host "`n=== 2. Creating External Worktrees (matching AO worktree architecture) ===" -ForegroundColor Cyan
for ($i = 1; $i -le $WorkerCount; $i++) {
    $branch = "feature/worker-$i"
    $wtPath = Join-Path $WorktreeBase "worker-$i"
    
    Write-Host "Creating worktree for Worker $i on branch $branch at $wtPath..."
    git -C $RepoPath branch $branch
    git -C $RepoPath worktree add $wtPath $branch
    
    # Simulate worker writing its own isolated module
    $moduleFile = Join-Path $wtPath "module_$i.txt"
    "Worker $i implementation content" | Set-Content $moduleFile
    git -C $wtPath add "module_$i.txt"
    git -C $wtPath commit -m "feat: worker $i isolated implementation"
}

Write-Host "`n=== 3. Validating Worktree Isolation & Zero Collision ===" -ForegroundColor Cyan
$mainStatus = git -C $RepoPath status --porcelain
if ($mainStatus) {
    Write-Host "[FAIL] Main repo working tree was dirtied by workers: $mainStatus" -ForegroundColor Red
    exit 1
} else {
    Write-Host "[PASS] GIT_WORKTREE_PRIMITIVE_PROVEN: Worktree isolation verified across $WorkerCount workers. Main working tree remains 100% clean." -ForegroundColor Green
}

$worktreeList = git -C $RepoPath worktree list
Write-Host "Registered Worktrees:`n$worktreeList" -ForegroundColor Green

Write-Host "`n=== 4. Cleaning up disposable repository ===" -ForegroundColor Cyan
for ($i = 1; $i -le $WorkerCount; $i++) {
    $wtPath = Join-Path $WorktreeBase "worker-$i"
    git -C $RepoPath worktree remove -f $wtPath 2>$null
}
Remove-Item -Path $RepoPath -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -Path $WorktreeBase -Recurse -Force -ErrorAction SilentlyContinue
Write-Host "[OK] Disposable repo and external worktrees cleaned up." -ForegroundColor Green
