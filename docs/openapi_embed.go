package docs

import _ "embed"

// OpenAPI3Spec 内嵌由 swagger2openapi 转换得到的 OpenAPI 3.0 规范文档
// 来源:执行 scripts/gen-docs.ps1 后在 docs/openapi.json 生成
// 注意:swag init 不会清空 docs 子目录下非自动生成的文件,但建议运行 scripts/gen-docs.ps1 而非单独执行 swag init
//go:embed openapi.json
var OpenAPI3Spec []byte
