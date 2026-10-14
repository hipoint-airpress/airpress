package filestorageimpl

import (
	"testing"
	"time"
)

func TestCategoryOf(t *testing.T) {
	cases := []struct {
		ext  string
		want string
	}{
		{"jpg", "img"},
		{"JPG", "img"},
		{".png", "img"},
		{"svg", "img"},
		// docx 本质是 zip 容器，按扩展名分类才能留在 doc，用 MIME 嗅探会被分到 zip
		{"docx", "doc"},
		{"xlsx", "doc"},
		{"pdf", "doc"},
		{"mp4", "video"},
		{"mkv", "video"},
		{"mp3", "audio"},
		{"flac", "audio"},
		{"7z", "zip"},
		{"tar", "zip"},
		{"exe", "file"},
		{"", "file"},
	}
	for _, c := range cases {
		if got := categoryOf(c.ext); got != c.want {
			t.Errorf("categoryOf(%q) = %q, want %q", c.ext, got, c.want)
		}
	}
}

func TestStorageSubPath(t *testing.T) {
	oct := time.Date(2026, 10, 13, 8, 0, 0, 0, time.Local)
	mar := time.Date(2026, 3, 5, 0, 0, 0, 0, time.Local)
	cases := []struct {
		name     string
		at       time.Time
		filename string
		want     string
	}{
		{"图片按年月分层", oct, "a.JPG", "img/2026/10"},
		{"文档", oct, "b.docx", "doc/2026/10"},
		{"未知扩展名兜底", oct, "c.dat", "file/2026/10"},
		{"无扩展名", oct, "README", "file/2026/10"},
		{"月份补零", mar, "d.mp3", "audio/2026/03"},
		{"浏览器可能带 Windows 路径", oct, `C:\fakepath\e.png`, "img/2026/10"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := storageSubPath(c.at, c.filename); got != c.want {
				t.Errorf("storageSubPath() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestCloudSubPath(t *testing.T) {
	oct := time.Date(2026, 10, 13, 8, 0, 0, 0, time.Local)
	cases := []struct {
		source string
		want   string
	}{
		{"", "img/2026/10"},
		{"upload", "upload/img/2026/10"},
		// 前缀与类型目录纯拼接：source 以 "img" 结尾也照常追加 "img/2026/10"，不做去重
		{"static/img", "static/img/img/2026/10"},
	}
	for _, c := range cases {
		if got := cloudSubPath(c.source, oct, "a.jpg"); got != c.want {
			t.Errorf("cloudSubPath(%q) = %q, want %q", c.source, got, c.want)
		}
	}
}
