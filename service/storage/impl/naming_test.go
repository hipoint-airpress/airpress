package filestorageimpl

import (
	"strings"
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

func TestRandomFilename(t *testing.T) {
	name, err := randomFilename()
	if err != nil {
		t.Fatalf("randomFilename() err = %v", err)
	}
	// 长度用硬编码字面量断言：若复用 nameSize，常量与断言会同向修改、测试恒绿，钉不住契约。
	// 与常量解耦正是目的，改 nameSize 时这里必须变红。
	if len(name) != 10 {
		t.Errorf("randomFilename() 长度 = %d, want 10（随机名长度契约）", len(name))
	}
	for _, r := range name {
		if !strings.ContainsRune(nameAlphabet, r) {
			t.Errorf("randomFilename() = %q 含非 base62 字符 %q", name, r)
		}
	}
	other, err := randomFilename()
	if err != nil {
		t.Fatalf("randomFilename() err = %v", err)
	}
	if name == other {
		t.Errorf("连续两次 randomFilename() 结果相同: %q", name)
	}
}

func TestUploadBaseName(t *testing.T) {
	cases := []struct {
		filename string
		want     string
	}{
		{"微信截图.jpg", "微信截图"},
		{"a.tar.gz", "a.tar"},
		{".htaccess", ".htaccess"},
		{"photo", "photo"},
		{`C:\fakepath\p.png`, "p"},
		{"dir/sub/p.png", "p"},
		{"", "attachment"},
		{strings.Repeat("名", 300) + ".txt", strings.Repeat("名", 255)},
	}
	for _, c := range cases {
		if got := uploadBaseName(c.filename); got != c.want {
			t.Errorf("uploadBaseName(%q) = %q, want %q", c.filename, got, c.want)
		}
	}
}

func TestUploadDisplayName(t *testing.T) {
	cases := []struct {
		filename string
		want     string
	}{
		{"微信截图.jpg", "微信截图.jpg"},
		{`C:\fakepath\p.png`, "p.png"},
		{"dir/sub/p.png", "p.png"},
		{"photo", "photo"},
		{".htaccess", ".htaccess"},
		{"", "attachment"},
	}
	for _, c := range cases {
		if got := uploadDisplayName(c.filename); got != c.want {
			t.Errorf("uploadDisplayName(%q) = %q, want %q", c.filename, got, c.want)
		}
	}
}

func TestContentDisposition(t *testing.T) {
	cases := []struct {
		filename string
		want     string
	}{
		{"report.pdf", `inline; filename="report.pdf"; filename*=UTF-8''report.pdf`},
		{"微信截图.jpg", `inline; filename="____.jpg"; filename*=UTF-8''%E5%BE%AE%E4%BF%A1%E6%88%AA%E5%9B%BE.jpg`},
		{`a"b.jpg`, `inline; filename="a_b.jpg"; filename*=UTF-8''a%22b.jpg`},
		{"空格 测试.txt", `inline; filename="__ __.txt"; filename*=UTF-8''%E7%A9%BA%E6%A0%BC%20%E6%B5%8B%E8%AF%95.txt`},
	}
	for _, c := range cases {
		if got := contentDisposition(c.filename); got != c.want {
			t.Errorf("contentDisposition(%q)\n got = %q\nwant = %q", c.filename, got, c.want)
		}
	}
}

func TestDocStorageBaseName(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		want     string
	}{
		{"中文与字母数字保留", "年度报告2026.docx", "年度报告2026"},
		{"去 Windows 上传路径", `C:\fakepath\报告.pdf`, "报告"},
		{"去 POSIX 上传路径", "dir/sub/报告.pdf", "报告"},
		{"危险字符替换为下划线", `a:b*c?d"e<f>g|h#i%j.pdf`, "a_b_c_d_e_f_g_h_i_j"},
		{"普通空格与制表符换行删除", "a b\tc\nd.pdf", "abcd"},
		{"不换行空格与全角空格删除", "a\u00A0b\u3000c.pdf", "abc"},
		{"零宽空格与 BOM 删除", "a\u200Bb\uFEFFc.pdf", "abc"},
		{"去首尾点", "...报告...pdf", "报告"},
		{"清洗后为空兜底", "....pdf", "attachment"},
		{"超长中文按字节截断不切坏汉字", strings.Repeat("名", 100) + ".pdf", strings.Repeat("名", 66)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := docStorageBaseName(c.filename); got != c.want {
				t.Errorf("docStorageBaseName(%q) = %q, want %q", c.filename, got, c.want)
			}
		})
	}
}
