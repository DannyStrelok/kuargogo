$ErrorActionPreference = "Stop"

# Retrieve the script location and navigate to the project root
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
# Assuming scripts/ is one level deep from root
$projectRoot = Split-Path -Parent $scriptDir 
Set-Location $projectRoot

Write-Host "🔍 Starting kuargogo Local Audit..." -ForegroundColor Cyan
Write-Host "📂 Working Directory: $(Get-Location)" -ForegroundColor DarkGray

# 1. Linter
Write-Host "`n👀 Running Linter..." -ForegroundColor Yellow
if (Get-Command "golangci-lint" -ErrorAction SilentlyContinue) {
    golangci-lint run ./...
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ Linter Passed" -ForegroundColor Green
    } else {
        Write-Error "❌ Linter Failed"
    }
} else {
    Write-Warning "golangci-lint not found. Skipping."
}

# 2. Tests
Write-Host "`n🧪 Running Tests..." -ForegroundColor Yellow

# Check if GCC is available for CGO (required for -race)
if (Get-Command "gcc" -ErrorAction SilentlyContinue) {
    $env:CGO_ENABLED = "1"
    Write-Host "   (Race detector enabled via GCC)" -ForegroundColor DarkGray
    go test -race -v ./...
} else {
    Write-Warning "GCC not found. Skipping race detector (-race). Install built-in tools (MinGW/TDM-GCC) to enable."
    $env:CGO_ENABLED = "0"
    go test -v ./...
}

if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Tests Passed" -ForegroundColor Green
} else {
    Write-Error "❌ Tests Failed"
}

# 3. Security (Trivy)
Write-Host "`n🛡️  Running Security Scan (Trivy)..." -ForegroundColor Yellow
if (Get-Command "trivy" -ErrorAction SilentlyContinue) {
    # Updated flags: --vuln-type is deprecated, use --pkg-types
    # Also ensure we scan the root '.'
    trivy fs . --severity CRITICAL,HIGH --ignore-unfixed --pkg-types os,library
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✅ Security Scan Passed" -ForegroundColor Green
    } else {
        Write-Error "❌ Security Scan Failed"
    }
} else {
    Write-Warning "Trivy not found. Skipping."
}

Write-Host "`n✨ Audit Complete!" -ForegroundColor Cyan
