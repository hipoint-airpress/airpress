# 生成 API 文档脚本
# 完整流程:
#   1. swag init      解析 handler 上的注释,生成 OpenAPI 2.0 (swagger.json + docs.go)
#   2. swagger2openapi  将 swagger.json 转换为 OpenAPI 3.0 (openapi.json)
#   3. go build       验证 //go:embed openapi.json 能正确内嵌
#
# 用法:
#   pwsh scripts/gen-docs.ps1            # 完整流程
#   pwsh scripts/gen-docs.ps1 -SkipBuild # 跳过最后的 build 验证
#
# 依赖:
#   - Go 工具链
#   - swag CLI     (go install github.com/swaggo/swag/cmd/swag@latest)
#   - Node.js + npm (用于 npx swagger2openapi)

[CmdletBinding()]
param(
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"
$projectRoot = Resolve-Path "$PSScriptRoot/.."
Set-Location $projectRoot

Write-Host "==> [1/3] swag init (生成 OpenAPI 2.0 docs/swagger.json)" -ForegroundColor Cyan
$swag = Join-Path (go env GOPATH) "bin\swag.exe"
if (-not (Test-Path $swag)) {
    Write-Error "swag CLI 未安装,请先执行: go install github.com/swaggo/swag/cmd/swag@latest"
    exit 1
}
& $swag init -g main.go -o docs --parseDependency --parseInternal --quiet
if ($LASTEXITCODE -ne 0) {
    Write-Error "swag init 失败 (exit $LASTEXITCODE)"
    exit $LASTEXITCODE
}

Write-Host "==> [2/3] swagger2openapi (转换为 OpenAPI 3.0 docs/openapi.json)" -ForegroundColor Cyan
npx --yes swagger2openapi@latest docs/swagger.json -o docs/openapi.json 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) {
    Write-Error "swagger2openapi 转换失败 (exit $LASTEXITCODE)"
    exit $LASTEXITCODE
}
$version = (Get-Content docs/openapi.json -Raw | ConvertFrom-Json).openapi
Write-Host "    openapi version: $version" -ForegroundColor Green

if ($SkipBuild) {
    Write-Host "==> [3/3] 跳过 go build (-SkipBuild)" -ForegroundColor Yellow
} else {
    Write-Host "==> [3/3] go build (验证 //go:embed openapi.json)" -ForegroundColor Cyan
    go build ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Error "go build 失败 (exit $LASTEXITCODE)"
        exit $LASTEXITCODE
    }
    Write-Host "    build OK" -ForegroundColor Green
}

Write-Host "`n完成。访问方式:" -ForegroundColor Green
Write-Host "  Swagger UI(2.0):    http://localhost:8080/swagger/index.html"
Write-Host "  OpenAPI 3.0 原始:   http://localhost:8080/openapi.json"
Write-Host "  3.0 文档可导入 Postman、Redoc、RapiDoc、Scalar 等工具"
