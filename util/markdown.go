package util

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// markdownConverter 开启 GFM 扩展(表格/删除线/任务列表/自动链接),
// goldmark 转换器并发安全,包级复用。
var markdownConverter = goldmark.New(goldmark.WithExtensions(extension.GFM))

// MarkdownToHTML 将 Markdown 源文本渲染为 HTML。
func MarkdownToHTML(source string) (string, error) {
	var buf bytes.Buffer
	if err := markdownConverter.Convert([]byte(source), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
