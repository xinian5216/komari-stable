# 薄包装：repo/tag/hash/下载/校验的全部逻辑都在 scripts/prepare-assets.py（跨平台单一事实来源）。
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

$python = $null
foreach ($candidate in @("python3", "python", "py")) {
    if (Get-Command $candidate -ErrorAction SilentlyContinue) {
        & $candidate -c "import zipfile" 2>$null
        if ($LASTEXITCODE -eq 0) { $python = $candidate; break }
    }
}
if (-not $python) { Write-Error "python3 (or python) with the zipfile module is required"; exit 1 }
& $python "scripts/prepare-assets.py" @args
exit $LASTEXITCODE
