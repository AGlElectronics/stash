<#
.SYNOPSIS
    Run the same lint pass CI runs against the local tree.

.DESCRIPTION
    Mirror of scripts/lint.sh for Windows / PowerShell. Lint runs in
    throwaway Docker images built from scripts/lint-*.Dockerfile so the
    host doesn't need Go, Node or golangci-lint installed.

    We use `docker build` (not `docker run -v`) because Docker Desktop on
    Windows can't bind-mount network drives reliably (the mount silently
    produces an empty dir inside the container).

    Backend lint also exports the regenerated sqlcgen back to the host
    via `docker buildx --output type=local`, which keeps IDE
    intellisense in sync after editing queries/migrations.

    Versions are pinned in the Dockerfiles to match the shared CI (module-ci.yml).

.PARAMETER Target
    backend | frontend | all (default: all)

.EXAMPLE
    ./scripts/lint.ps1
    ./scripts/lint.ps1 backend
#>
[CmdletBinding()]
param(
    [ValidateSet('backend', 'frontend', 'all')]
    [string]$Target = 'all'
)

# Don't promote native command stderr to a PowerShell error stream. Docker
# buildx writes its progress UI to stderr and Windows PowerShell 5.1 would
# otherwise abort on the first line. We check $LASTEXITCODE manually.
$ErrorActionPreference = 'Continue'

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$backendDockerfile  = Join-Path $PSScriptRoot 'lint-backend.Dockerfile'
$frontendDockerfile = Join-Path $PSScriptRoot 'lint-frontend.Dockerfile'

function Invoke-Docker {
    # Merge stderr into stdout so PS5.1 doesn't treat Docker's progress UI
    # (written to stderr) as errors. Bubbles up exit code.
    param([Parameter(ValueFromRemainingArguments = $true)] [string[]] $Args)
    & cmd /c "docker $($Args -join ' ') 2>&1"
}

function Invoke-BackendLint {
    Write-Host "==> backend: sqlc generate + golangci-lint (docker build)" -ForegroundColor Cyan
    $sqlcgenOut  = Join-Path $repoRoot 'backend\internal\repository\sqlcgen'
    $backendCtx  = Join-Path $repoRoot 'backend'
    Invoke-Docker buildx build `
        --target sqlcgen-export `
        --output "type=local,dest=`"$sqlcgenOut`"" `
        -f "`"$backendDockerfile`"" `
        "`"$backendCtx`""
    if ($LASTEXITCODE -ne 0) { throw "backend lint failed (exit $LASTEXITCODE)" }
}

function Invoke-FrontendLint {
    Write-Host "==> frontend: eslint + tsc (docker build)" -ForegroundColor Cyan
    $frontendCtx = Join-Path $repoRoot 'frontend'
    Invoke-Docker build `
        -f "`"$frontendDockerfile`"" `
        "`"$frontendCtx`""
    if ($LASTEXITCODE -ne 0) { throw "frontend lint failed (exit $LASTEXITCODE)" }
}

switch ($Target) {
    'backend'  { Invoke-BackendLint }
    'frontend' { Invoke-FrontendLint }
    'all'      { Invoke-BackendLint; Invoke-FrontendLint }
}

Write-Host "==> lint OK" -ForegroundColor Green
