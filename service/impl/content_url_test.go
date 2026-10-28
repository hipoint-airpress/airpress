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
