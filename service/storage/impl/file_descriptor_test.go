package filestorageimpl

import (
	"errors"
	"strings"
	"testing"
)

// 核心：改名策略先执行，二次查重必须拿策略改过之后的路径去判断
func TestRenameStrategyRunsBeforeDupCheck(t *testing.T) {
	var checked string
	fd, err := newURLFileDescriptor(
		withBaseURL("https://bucket.oss-cn-hangzhou.aliyuncs.com"),
		withSubURLPath("img/2026/10"),
		withOriginalNameURLOption("photo.jpg"),
		withShouldRenameURLOption(func(relativePath string) (bool, error) {
			checked = relativePath
			return true, nil
		}),
		withRenameURLOption(func() (string, error) { return "24u24dfads", nil }),
	)
	if err != nil {
		t.Fatalf("newURLFileDescriptor() err = %v", err)
	}
	if checked != "img/2026/10/24u24dfads.jpg" {
		t.Errorf("shouldRename 收到的应是改名后的路径, got %q, want %q", checked, "img/2026/10/24u24dfads.jpg")
	}
	if !strings.HasPrefix(fd.getFileName(), "24u24dfads-") {
		t.Errorf("判定重复后应追加时间戳, getFileName() = %q", fd.getFileName())
	}
	if fd.getExtensionName() != "jpg" {
		t.Errorf("getExtensionName() = %q, want %q", fd.getExtensionName(), "jpg")
	}
}

// 策略改完不重复时不加时间戳
func TestRenameStrategyWithoutDup(t *testing.T) {
	fd, err := newURLFileDescriptor(
		withBaseURL("https://example.com"),
		withSubURLPath("img/2026/10"),
		withOriginalNameURLOption("photo.jpg"),
		withShouldRenameURLOption(func(string) (bool, error) { return false, nil }),
		withRenameURLOption(func() (string, error) { return "24u24dfads", nil }),
	)
	if err != nil {
		t.Fatalf("newURLFileDescriptor() err = %v", err)
	}
	if got := fd.getRelativePath(); got != "img/2026/10/24u24dfads.jpg" {
		t.Errorf("getRelativePath() = %q, want %q", got, "img/2026/10/24u24dfads.jpg")
	}
}

// 没传策略时保持改造前的默认行为：沿用原始基名
func TestDefaultBehaviorWithoutRename(t *testing.T) {
	fd, err := newURLFileDescriptor(
		withBaseURL("https://example.com"),
		withSubURLPath("img/2026/10"),
		withOriginalNameURLOption("photo.jpg"),
	)
	if err != nil {
		t.Fatalf("newURLFileDescriptor() err = %v", err)
	}
	if got := fd.getRelativePath(); got != "img/2026/10/photo.jpg" {
		t.Errorf("未指定策略时应沿用原始文件名, getRelativePath() = %q", got)
	}
}

// 没传 shouldRename 时只执行策略，不做二次查重
func TestRenameStrategyWithoutDupCheck(t *testing.T) {
	fd, err := newURLFileDescriptor(
		withBaseURL("https://example.com"),
		withSubURLPath(""),
		withOriginalNameURLOption("photo.jpg"),
		withRenameURLOption(func() (string, error) { return "24u24dfads", nil }),
	)
	if err != nil {
		t.Fatalf("newURLFileDescriptor() err = %v", err)
	}
	if got := fd.getRelativePath(); got != "24u24dfads.jpg" {
		t.Errorf("空前缀不应产生多余层级, getRelativePath() = %q", got)
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

// 本地 fd 不传策略时的默认行为不受影响
func TestLocalDefaultBehaviorWithoutRename(t *testing.T) {
	fd, err := newLocalFileDescriptor(
		withOriginalName("photo.png"),
		withBasePath("/data/airpress"),
		withSubPath("upload/img/2026/10"),
	)
	if err != nil {
		t.Fatalf("newLocalFileDescriptor() err = %v", err)
	}
	if got := fd.getRelativePath(); got != "upload/img/2026/10/photo.png" {
		t.Errorf("getRelativePath() = %q, want %q", got, "upload/img/2026/10/photo.png")
	}
}
