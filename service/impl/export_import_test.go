package impl

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/hipoint-airpress/airpress/config"
	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/entity"
)

// useTempBackupDir 将 Markdown 备份目录指向临时目录，避免测试污染默认目录。
func useTempBackupDir(t *testing.T) {
	t.Helper()
	prev := config.BackupMarkdownDir
	config.BackupMarkdownDir = t.TempDir()
	t.Cleanup(func() {
		config.BackupMarkdownDir = prev
	})
}

// TestExportMarkdownResolvesAttachmentURLs 验证导出的 Markdown 中附件相对路径被解析为完整 URL。
func TestExportMarkdownResolvesAttachmentURLs(t *testing.T) {
	db := setupPostAttachmentDB(t)
	if err := db.Create(&entity.Attachment{
		Name: "a", Path: "upload/img/a.png", FileKey: "upload/img/a.png", Type: consts.AttachmentTypeLocal,
	}).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	if err := db.Create(&entity.Post{
		Type:            consts.PostTypePost,
		CreateTime:      time.Now(),
		Slug:            "resolved",
		Status:          consts.PostStatusPublished,
		OriginalContent: "![a](upload/img/a.png)",
	}).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}
	useTempBackupDir(t)

	composite := fakeFileStorageComposite{
		consts.AttachmentTypeLocal: &fakeFileStorage{attachmentType: consts.AttachmentTypeLocal, baseURL: "https://blog.example.com"},
	}
	svc := NewExportImport(nil, &postServiceImpl{}, nil, nil, nil, NewContentURLService(composite))

	backupPath, err := svc.ExportMarkdown(context.Background(), false)
	if err != nil {
		t.Fatalf("ExportMarkdown() error = %v", err)
	}

	zr, err := zip.OpenReader(backupPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer func() { _ = zr.Close() }()

	if len(zr.File) != 1 {
		t.Fatalf("zip entry count = %d, want 1", len(zr.File))
	}
	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatalf("open zip entry: %v", err)
	}
	defer func() { _ = rc.Close() }()
	content, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read zip entry: %v", err)
	}

	want := "![a](https://blog.example.com/upload/img/a.png)"
	if string(content) != want {
		t.Fatalf("markdown content = %q, want %q", string(content), want)
	}
}

// failingContentURLService 让 Resolve 固定返回错误，用于验证导出路径的错误传播。
type failingContentURLService struct {
	stubContentURLService
	err error
}

func (f failingContentURLService) Resolve(context.Context, string) (string, error) {
	return "", f.err
}

// TestExportMarkdownReturnsResolveError 验证 Resolve 失败时导出直接返回错误。
func TestExportMarkdownReturnsResolveError(t *testing.T) {
	db := setupPostAttachmentDB(t)
	if err := db.Create(&entity.Post{
		Type:            consts.PostTypePost,
		CreateTime:      time.Now(),
		Slug:            "resolve-error",
		Status:          consts.PostStatusPublished,
		OriginalContent: "![a](upload/img/a.png)",
	}).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}
	useTempBackupDir(t)

	wantErr := errors.New("resolve failed")
	svc := NewExportImport(nil, &postServiceImpl{}, nil, nil, nil, failingContentURLService{err: wantErr})

	_, err := svc.ExportMarkdown(context.Background(), false)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ExportMarkdown() error = %v, want %v", err, wantErr)
	}
}
