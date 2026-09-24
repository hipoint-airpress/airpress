package util

import (
	"strings"
	"testing"
)

func TestMarkdownToHTML(t *testing.T) {
	cases := []struct {
		name string
		md   string
		want []string // 输出必须包含的 HTML 片段
	}{
		{"标题与粗体", "# 标题\n\n**粗体**", []string{"<h1>标题</h1>", "<strong>粗体</strong>"}},
		{"GFM 表格", "| a |\n| --- |\n| 1 |", []string{"<table>", "<td>1</td>"}},
		{"围栏代码块", "```\ncode\n```", []string{"<pre>", "<code>code\n</code>"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := MarkdownToHTML(c.md)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Errorf("output %q missing %q", got, w)
				}
			}
		})
	}
}

func TestMarkdownToHTMLEmpty(t *testing.T) {
	got, err := MarkdownToHTML("")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty output, got %q", got)
	}
}
