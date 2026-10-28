package service

import "context"

// ContentURLService 在内容写入与输出边界转换正文中的附件 URL。
type ContentURLService interface {
	// Normalize 将正文中指向附件库的完整 URL 归一化为 attachment.path，幂等。
	Normalize(ctx context.Context, content string) (string, error)
	// Resolve 将正文中的附件相对 path 解析为当前存储配置下的完整 URL，幂等。
	Resolve(ctx context.Context, content string) (string, error)
}
