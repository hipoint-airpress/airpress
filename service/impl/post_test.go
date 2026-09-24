package impl

import (
	"context"
	"strings"
	"testing"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/param"
)

func ptrEditor(e consts.EditorType) *consts.EditorType { return &e }

// markdown 编辑器写入时,服务端应将 OriginalContent 渲染为 HTML 存入 FormatContent,
// 而不是原样信任客户端提交的 Content。
func TestPostConvertParamMarkdownRendersHTML(t *testing.T) {
	md := "# 标题\n\n**粗体**"
	post, err := postServiceImpl{}.ConvertParam(context.Background(), &param.Post{
		Title:           "t",
		EditorType:      ptrEditor(consts.EditorTypeMarkdown),
		OriginalContent: md,
		Content:         md, // 模拟客户端未转换、原样提交了 markdown
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(post.FormatContent, "<h1>标题</h1>") {
		t.Errorf("FormatContent 应为渲染后的 HTML,实际: %q", post.FormatContent)
	}
	if post.OriginalContent != md {
		t.Errorf("OriginalContent 应保持 markdown 原文,实际: %q", post.OriginalContent)
	}
}

// 富文本写入保持原样透传 Content,不做二次渲染。
func TestPostConvertParamRichTextPassthrough(t *testing.T) {
	html := "<p>已排版</p>"
	post, err := postServiceImpl{}.ConvertParam(context.Background(), &param.Post{
		Title:           "t",
		EditorType:      ptrEditor(consts.EditorTypeRichText),
		OriginalContent: html,
		Content:         html,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if post.FormatContent != html {
		t.Errorf("富文本应透传,实际: %q", post.FormatContent)
	}
}

// markdown 类型但原文为空时,回退透传 Content(兼容只提交 HTML 的调用方)。
func TestPostConvertParamMarkdownEmptyOriginalFallsBack(t *testing.T) {
	client := "<p>from client</p>"
	post, err := postServiceImpl{}.ConvertParam(context.Background(), &param.Post{
		Title:      "t",
		EditorType: ptrEditor(consts.EditorTypeMarkdown),
		Content:    client,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if post.FormatContent != client {
		t.Errorf("原文为空应透传 Content,实际: %q", post.FormatContent)
	}
}

// sheet 与 post 走同样的写入策略。
func TestSheetConvertParamMarkdownRendersHTML(t *testing.T) {
	md := "## 页面\n\n- 列表项"
	sheet, err := sheetServiceImpl{}.ConvertParam(context.Background(), &param.Sheet{
		Title:           "s",
		EditorType:      ptrEditor(consts.EditorTypeMarkdown),
		OriginalContent: md,
		Content:         md,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(sheet.FormatContent, "<h2>页面</h2>") || !strings.Contains(sheet.FormatContent, "<li>列表项</li>") {
		t.Errorf("sheet FormatContent 应为渲染后的 HTML,实际: %q", sheet.FormatContent)
	}
}
