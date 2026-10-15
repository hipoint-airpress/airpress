package filestorageimpl

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

// randomNamePattern 钉住默认改名契约：10 位 base62 随机名主体
var randomNamePattern = regexp.MustCompile(`^[0-9A-Za-z]{10}$`)

// 改名策略生效：文件主体名取策略返回值，扩展名仍来自原始文件名
func TestRenameStrategySetsName(t *testing.T) {
	fd, err := newURLFileDescriptor(
		withBaseURL("https://bucket.oss-cn-hangzhou.aliyuncs.com"),
		withSubURLPath("img/2026/10"),
		withOriginalNameURLOption("photo.jpg"),
		withRenameURLOption(func() (string, error) { return "24u24dfads", nil }),
	)
	if err != nil {
		t.Fatalf("newURLFileDescriptor() err = %v", err)
	}
	if got := fd.getFileName(); got != "24u24dfads" {
		t.Errorf("getFileName() = %q, want %q", got, "24u24dfads")
	}
	if got := fd.getRelativePath(); got != "img/2026/10/24u24dfads.jpg" {
		t.Errorf("getRelativePath() = %q, want %q", got, "img/2026/10/24u24dfads.jpg")
	}
	if got := fd.getExtensionName(); got != "jpg" {
		t.Errorf("getExtensionName() = %q, want %q", got, "jpg")
	}
}

// 未指定改名策略时默认用 randomFilename 生成随机名，且仍保留原始扩展名与目录前缀
func TestURLDefaultRenameIsRandom(t *testing.T) {
	newFD := func() fileDescriptor {
		fd, err := newURLFileDescriptor(
			withBaseURL("https://example.com"),
			withSubURLPath("img/2026/10"),
			withOriginalNameURLOption("photo.jpg"),
		)
		if err != nil {
			t.Fatalf("newURLFileDescriptor() err = %v", err)
		}
		return fd
	}
	first, second := newFD(), newFD()

	if !randomNamePattern.MatchString(first.getFileName()) {
		t.Errorf("默认改名应为 10 位 base62 随机名, getFileName() = %q", first.getFileName())
	}
	if got := first.getExtensionName(); got != "jpg" {
		t.Errorf("getExtensionName() = %q, want %q", got, "jpg")
	}
	if !strings.HasPrefix(first.getRelativePath(), "img/2026/10/") || !strings.HasSuffix(first.getRelativePath(), ".jpg") {
		t.Errorf("默认改名不应丢失前缀或扩展名, getRelativePath() = %q", first.getRelativePath())
	}
	if first.getFileName() == second.getFileName() {
		t.Errorf("两次默认改名的结果相同: %q", first.getFileName())
	}
}

// 只执行改名策略；目录前缀为空时不应产生多余层级
func TestRenameStrategyOnly(t *testing.T) {
	cases := []struct {
		name    string
		subPath string
		want    string
	}{
		{"带类型年月前缀", "img/2026/10", "img/2026/10/24u24dfads.jpg"},
		{"空前缀不产生多余层级", "", "24u24dfads.jpg"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fd, err := newURLFileDescriptor(
				withBaseURL("https://example.com"),
				withSubURLPath(c.subPath),
				withOriginalNameURLOption("photo.jpg"),
				withRenameURLOption(func() (string, error) { return "24u24dfads", nil }),
			)
			if err != nil {
				t.Fatalf("newURLFileDescriptor() err = %v", err)
			}
			if got := fd.getRelativePath(); got != c.want {
				t.Errorf("getRelativePath() = %q, want %q", got, c.want)
			}
		})
	}
}

// 策略报错必须向上冒泡
func TestRenameStrategyErrorPropagates(t *testing.T) {
	want := errors.New("boom")
	_, err := newURLFileDescriptor(
		withBaseURL("https://example.com"),
		withSubURLPath("img/2026/10"),
		withOriginalNameURLOption("photo.jpg"),
		withRenameURLOption(func() (string, error) { return "", want }),
	)
	if !errors.Is(err, want) {
		t.Errorf("newURLFileDescriptor() err = %v, want %v", err, want)
	}
}

// 本地 fd：主图与缩略图共用同一策略结果，且 -thumbnail 落在扩展名之前
func TestLocalRenameStrategySharedByThumbnail(t *testing.T) {
	name := "24u24dfads"
	fixed := func() (string, error) { return name, nil }
	opts := []fdOption{
		withOriginalName("photo.png"),
		withBasePath("/data/airpress"),
		withSubPath("upload/img/2026/10"),
	}
	main, err := newLocalFileDescriptor(append(opts, withRename(fixed))...)
	if err != nil {
		t.Fatalf("newLocalFileDescriptor() err = %v", err)
	}
	thumb, err := newLocalFileDescriptor(append(opts, withRename(fixed), withSuffix("-thumbnail"))...)
	if err != nil {
		t.Fatalf("newLocalFileDescriptor() err = %v", err)
	}
	if got := main.getRelativePath(); got != "upload/img/2026/10/24u24dfads.png" {
		t.Errorf("主图 getRelativePath() = %q", got)
	}
	// LocalFileStorage.Delete 靠「文件基名 + -thumbnail」推导缩略图路径，该关系必须成立
	if got := thumb.getRelativePath(); got != "upload/img/2026/10/24u24dfads-thumbnail.png" {
		t.Errorf("缩略图 getRelativePath() = %q", got)
	}
}

// 核心机制：缩略图策略读主图 fd 的名字（LocalFileStorage.Upload 的缩略图 fd 就是这么构造的）。
// 回调在主图 fd 构造完成之后才执行，因此取到的是主图最终采用的名字，
// 保证 LocalFileStorage.Delete 的「主图基名 + -thumbnail」推导始终成立。
func TestLocalThumbnailFollowsMainName(t *testing.T) {
	opts := []fdOption{
		withOriginalName("photo.png"),
		withBasePath("/data/airpress"),
		withSubPath("upload/img/2026/10"),
	}
	main, err := newLocalFileDescriptor(append(opts,
		withRename(func() (string, error) { return "24u24dfads", nil }),
	)...)
	if err != nil {
		t.Fatalf("newLocalFileDescriptor() main err = %v", err)
	}
	if got := main.getFileName(); got != "24u24dfads" {
		t.Fatalf("前置条件不成立：主图名应为策略返回值, getFileName() = %q", got)
	}
	thumb, err := newLocalFileDescriptor(append(opts,
		withRename(func() (string, error) { return main.getFileName(), nil }),
		withSuffix("-thumbnail"),
	)...)
	if err != nil {
		t.Fatalf("newLocalFileDescriptor() thumb err = %v", err)
	}
	want := strings.TrimSuffix(main.getRelativePath(), ".png") + "-thumbnail.png"
	if got := thumb.getRelativePath(); got != want {
		t.Errorf("缩略图 getRelativePath() = %q, want 跟随主图名 %q", got, want)
	}
}

// 本地 fd 未指定策略时同样默认随机改名
func TestLocalDefaultRenameIsRandom(t *testing.T) {
	fd, err := newLocalFileDescriptor(
		withOriginalName("photo.png"),
		withBasePath("/data/airpress"),
		withSubPath("upload/img/2026/10"),
	)
	if err != nil {
		t.Fatalf("newLocalFileDescriptor() err = %v", err)
	}
	if !randomNamePattern.MatchString(fd.getFileName()) {
		t.Errorf("默认改名应为 10 位 base62 随机名, getFileName() = %q", fd.getFileName())
	}
	if got := fd.getExtensionName(); got != "png" {
		t.Errorf("getExtensionName() = %q, want %q", got, "png")
	}
}
