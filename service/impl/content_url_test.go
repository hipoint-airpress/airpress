package impl

import (
	"reflect"
	"testing"
)

func TestExtractContentTargets(t *testing.T) {
	content := `![a](upload/a.png)
<img src="/upload/b.png">
[c](https://example.com/c.png "title")
[d](upload/a.png)`
	// 实现先扫描 HTML 属性、再扫描 Markdown，去重顺序与功能无关。
	want := []string{"/upload/b.png", "upload/a.png", "https://example.com/c.png"}
	if got := extractContentTargets(content); !reflect.DeepEqual(got, want) {
		t.Fatalf("extractContentTargets() = %#v, want %#v", got, want)
	}
}

func TestReplaceContentTargets(t *testing.T) {
	content := `![a](old/a.png)
<img src="old/b.png"><img src='old/c.png'>`
	replacements := map[string]string{
		"old/a.png": "new/a.png",
		"old/b.png": "new/b.png",
		"old/c.png": "new/c.png",
	}
	want := `![a](new/a.png)
<img src="new/b.png"><img src='new/c.png'>`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestExtractContentTargetsSingleQuotedHTML 覆盖单引号 HTML 属性（extract 的 match[5] 分支）。
func TestExtractContentTargetsSingleQuotedHTML(t *testing.T) {
	content := `<a href='sq.png'>x</a>`
	want := []string{"sq.png"}
	if got := extractContentTargets(content); !reflect.DeepEqual(got, want) {
		t.Fatalf("extractContentTargets() = %#v, want %#v", got, want)
	}
}

func TestExtractContentTargetsEmptyContent(t *testing.T) {
	if got := extractContentTargets(""); len(got) != 0 {
		t.Fatalf("extractContentTargets(\"\") = %#v, want empty", got)
	}
}

// TestReplaceContentTargetsNoMatch 验证未命中项（HTML 与 Markdown）原样返回。
func TestReplaceContentTargetsNoMatch(t *testing.T) {
	content := `<img src="miss.png"><a href='miss2.png'>x</a>[y](miss3.png)`
	replacements := map[string]string{"other.png": "new.png"}
	if got := replaceContentTargets(content, replacements); got != content {
		t.Fatalf("replaceContentTargets() = %q, want unchanged %q", got, content)
	}
}

// TestReplaceContentTargetsTrimsWhitespace 验证属性值首尾空白与提取时的裁剪一致。
func TestReplaceContentTargetsTrimsWhitespace(t *testing.T) {
	content := `<img src=" old.png ">`
	replacements := map[string]string{"old.png": "new.png"}
	want := `<img src="new.png">`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestExtractAndReplaceRoundTripTrimmedHTML 验证提取结果可直接用于替换。
func TestExtractAndReplaceRoundTripTrimmedHTML(t *testing.T) {
	content := `<img src=" old.png ">`
	targets := extractContentTargets(content)
	wantTargets := []string{"old.png"}
	if !reflect.DeepEqual(targets, wantTargets) {
		t.Fatalf("extractContentTargets() = %#v, want %#v", targets, wantTargets)
	}
	replacements := make(map[string]string, len(targets))
	for _, target := range targets {
		replacements[target] = "new.png"
	}
	want := `<img src="new.png">`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestExtractAndReplaceRoundTripTrimmedMarkdown 记录带空白 Markdown 目标的现状：
// 正则的前后捕获组会消费首尾空白，故提取结果为 old.png，替换后空白保留。
func TestExtractAndReplaceRoundTripTrimmedMarkdown(t *testing.T) {
	content := `![a]( old.png )`
	targets := extractContentTargets(content)
	wantTargets := []string{"old.png"}
	if !reflect.DeepEqual(targets, wantTargets) {
		t.Fatalf("extractContentTargets() = %#v, want %#v", targets, wantTargets)
	}
	replacements := map[string]string{"old.png": "new.png"}
	want := `![a]( new.png )`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestReplaceContentTargetsPreservesDataSrcAttribute 验证 data-src 前缀不被破坏。
func TestReplaceContentTargetsPreservesDataSrcAttribute(t *testing.T) {
	content := `<img data-src="old.png">`
	replacements := map[string]string{"old.png": "new.png"}
	want := `<img data-src="new.png">`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestReplaceContentTargetsPreservesMarkdownTitle 验证 Markdown title 保留。
func TestReplaceContentTargetsPreservesMarkdownTitle(t *testing.T) {
	content := `[c](old.png "title")`
	replacements := map[string]string{"old.png": "new.png"}
	want := `[c](new.png "title")`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestReplaceContentTargetsIgnoresPlainText 验证正文纯文本中的同名 URL 不被替换。
func TestReplaceContentTargetsIgnoresPlainText(t *testing.T) {
	content := `正文提到 old.png，但这不是链接。`
	replacements := map[string]string{"old.png": "new.png"}
	if got := replaceContentTargets(content, replacements); got != content {
		t.Fatalf("replaceContentTargets() = %q, want unchanged %q", got, content)
	}
}

// TestReplaceContentTargetsEscapesHTMLEntities 验证替换值在 HTML 属性中的转义。
func TestReplaceContentTargetsEscapesHTMLEntities(t *testing.T) {
	content := `<img src="old.png">`
	replacements := map[string]string{"old.png": "new.png?a=1&b=2"}
	want := `<img src="new.png?a=1&amp;b=2">`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestReplaceContentTargetsUppercaseAttribute 验证大写属性名可被替换。
func TestReplaceContentTargetsUppercaseAttribute(t *testing.T) {
	content := `<img SRC="old.png">`
	replacements := map[string]string{"old.png": "new.png"}
	want := `<img SRC="new.png">`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}

// TestReplaceContentTargetsHref 验证 href 属性可被替换。
func TestReplaceContentTargetsHref(t *testing.T) {
	content := `<a href="old.png">link</a>`
	replacements := map[string]string{"old.png": "new.png"}
	want := `<a href="new.png">link</a>`
	if got := replaceContentTargets(content, replacements); got != want {
		t.Fatalf("replaceContentTargets() = %q, want %q", got, want)
	}
}
