# 使用项目配置，不覆盖 registry；原生命令非零必须失败。
param([Parameter(Mandatory=$true)][string]$ProjectRoot)
$ErrorActionPreference = "Stop"
Push-Location -LiteralPath $ProjectRoot
try {
    if (-not (Test-Path -LiteralPath './hvigorw.bat')) { throw 'Missing project hvigorw.bat wrapper' }
    Get-Command ohpm -ErrorAction Stop | Out-Null
    ohpm install --all
    if ($LASTEXITCODE -ne 0) { throw "ohpm failed: $LASTEXITCODE" }
    & './hvigorw.bat' assembleApp
    if ($LASTEXITCODE -ne 0) { throw "hvigorw failed: $LASTEXITCODE" }
} finally {
    Pop-Location
}
