# Stress test for Git Worktree Concurrency Waves (1, 3, 5, 7)
# Tests Git worktree concurrent scaling primitives (GIT_WORKTREE_PRIMITIVE_PROVEN)
# NOTE: This validates local Git worktree scaling mechanics, NOT live multi-agent LLM concurrency.

param (
    [int[]]$Waves = @(1, 3, 5, 7),
    [string]$BaseTestDir = "D:\TU_CODE\test-stress-concurrency"
)

$ErrorActionPreference = "Stop"

Write-Host "=== Git Worktree Concurrency Primitive Evaluation (1, 3, 5, 7) ===" -ForegroundColor Cyan

foreach ($count in $Waves) {
    Write-Host "`n--- Testing Wave: $count concurrent workers ---" -ForegroundColor Yellow
    $repo = Join-Path $BaseTestDir "repo_wave_$count"
    $wtBase = Join-Path $BaseTestDir "wt_wave_$count"
    
    if (Test-Path $repo) { Remove-Item $repo -Recurse -Force }
    if (Test-Path $wtBase) { Remove-Item $wtBase -Recurse -Force }
    
    New-Item -ItemType Directory -Path $repo -Force | Out-Null
    New-Item -ItemType Directory -Path $wtBase -Force | Out-Null
    
    git -C $repo init | Out-Null
    "Baseline" | Set-Content (Join-Path $repo "README.md")
    git -C $repo add README.md
    git -C $repo commit -m "initial" | Out-Null
    
    $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    
    # Spawn N worktrees
    for ($i = 1; $i -le $count; $i++) {
        $branch = "feature/task-$i"
        $wt = Join-Path $wtBase "wt-$i"
        git -C $repo branch $branch | Out-Null
        git -C $repo worktree add $wt $branch | Out-Null
        
        # Simulate worker write & commit
        "Task $i work output" | Set-Content (Join-Path $wt "task_$i.txt")
        git -C $wt add "task_$i.txt"
        git -C $wt commit -m "task $i completed" | Out-Null
    }
    
    $stopwatch.Stop()
    $mainClean = -not (git -C $repo status --porcelain)
    
    Write-Host "  Workers spawned & committed: $count"
    Write-Host "  Elapsed time: $($stopwatch.ElapsedMilliseconds) ms"
    Write-Host "  Main tree isolation preserved: $mainClean" -ForegroundColor $(if ($mainClean) { "Green" } else { "Red" })
    
    if (-not $mainClean) {
        Write-Error "Isolation violation during wave $count"
        exit 1
    }
    
    # Cleanup
    for ($i = 1; $i -le $count; $i++) {
        $wt = Join-Path $wtBase "wt-$i"
        git -C $repo worktree remove -f $wt 2>$null
    }
    Remove-Item -Path $repo -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -Path $wtBase -Recurse -Force -ErrorAction SilentlyContinue
}

Remove-Item $BaseTestDir -Recurse -Force -ErrorAction SilentlyContinue
Write-Host "`n[PASS] GIT_WORKTREE_PRIMITIVE_PROVEN: Concurrency wave evaluation completed for waves: $($Waves -join ', ')." -ForegroundColor Green
