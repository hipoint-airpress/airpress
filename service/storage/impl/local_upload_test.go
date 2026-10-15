package filestorageimpl

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hipoint-airpress/airpress/config"
	"github.com/hipoint-airpress/airpress/model/dto"
)

// 本文件端到端验证 LocalFileStorage.Upload：真实 multipart、真实落盘。
// 驱动层不碰数据库（冲突防护在存储层：O_CREATE|O_EXCL，见 local.go 的 Upload），
// 因此这里不需要 SQLite；DTO→entity 各列的入库正确性由
// service/impl/attachment_upload_test.go 用真实 gorm 覆盖。

// uploadPathPattern 钉住落盘契约：upload/{类型}/{年}/{月}/{10 位 base62 随机名}.png
var uploadPathPattern = regexp.MustCompile(`^upload/img/[0-9]{4}/[0-9]{2}/[0-9A-Za-z]{10}\.png$`)

// setupLocalUpload 返回以 t.TempDir 为 WorkDir 的本地存储，不触碰仓库的 airpress.db 与 upload/。
func setupLocalUpload(t *testing.T) (*LocalFileStorage, string) {
	t.Helper()

	cfg := &config.Config{}
	workDir := t.TempDir()
	cfg.AirPress.WorkDir = workDir
	// Upload 路径不触碰 OptionService（只有 GetFilePath 用它），传 nil 即可
	return NewLocalFileStorage(cfg, nil), workDir
}

// pngFileHeader 走真实的 multipart 编解码拿到 *multipart.FileHeader；
// form 随测试结束 RemoveAll（不能用 defer——Upload 还要读它）。
func pngFileHeader(t *testing.T, filename string) *multipart.FileHeader {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(pngBuf.Bytes()); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	// NewReader 的第二个参数是裸 boundary（FormDataContentType() 是整个 Content-Type，需先解析）
	_, params, err := mime.ParseMediaType(w.FormDataContentType())
	if err != nil {
		t.Fatalf("ParseMediaType: %v", err)
	}
	form, err := multipart.NewReader(&body, params["boundary"]).ReadForm(1 << 20)
	if err != nil {
		t.Fatalf("ReadForm: %v", err)
	}
	t.Cleanup(func() { _ = form.RemoveAll() })
	fhs := form.File["file"]
	if len(fhs) != 1 {
		t.Fatalf("want exactly 1 file header, got %d", len(fhs))
	}
	return fhs[0]
}

// 场景一：中文命名 PNG 上传成功并正确落盘，随机名与主图/缩略图配对关系成立
func TestLocalUploadChinesePngSavesAndPersists(t *testing.T) {
	storage, workDir := setupLocalUpload(t)

	att, err := storage.Upload(context.Background(), pngFileHeader(t, "微信截图.png"))
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if att.Name != "微信截图" {
		t.Errorf("Name = %q, want 原始基名 %q", att.Name, "微信截图")
	}
	// att.Path 由 filepath.Join 产出，Windows 下是反斜杠，断言前先斜杠化
	if !uploadPathPattern.MatchString(filepath.ToSlash(att.Path)) {
		t.Errorf("Path = %q, 不符合落盘契约 %q", att.Path, uploadPathPattern.String())
	}
	// 说明：该断言与 local.go 中 FileKey 的取值是同一表达式，只能防未来二者分叉，
	// 不能证明 key 生成逻辑本身正确
	if att.FileKey != att.Path {
		t.Errorf("FileKey = %q, want 与 Path 相同 %q", att.FileKey, att.Path)
	}
	if !strings.Contains(att.MediaType, "image/png") {
		t.Errorf("MediaType = %q, want 含 image/png", att.MediaType)
	}
	if att.Suffix != "png" {
		t.Errorf("Suffix = %q, want %q", att.Suffix, "png")
	}
	if att.Width != 2 || att.Height != 2 {
		t.Errorf("Width/Height = %d/%d, want 2/2", att.Width, att.Height)
	}
	if att.Size == 0 {
		t.Errorf("Size = %d, want 非零（真实 PNG 字节数）", att.Size)
	}

	wantThumb := strings.TrimSuffix(att.Path, ".png") + "-thumbnail.png"
	if att.ThumbPath != wantThumb {
		t.Errorf("ThumbPath = %q, want 主图基名 + -thumbnail 后缀 %q", att.ThumbPath, wantThumb)
	}

	// 主图与缩略图必须真的落盘
	for _, p := range []string{att.Path, att.ThumbPath} {
		full := filepath.Join(workDir, filepath.FromSlash(p))
		st, err := os.Stat(full)
		if err != nil {
			t.Errorf("落盘文件缺失 %s: %v", full, err)
			continue
		}
		if st.Size() == 0 {
			t.Errorf("落盘文件为空 %s", full)
		}
	}
}

// 场景二：同名二次上传得到不同随机名，两次都保留原始基名。
// 注意：本文件不覆盖「同名冲突」分支——10 位 base62 随机名撞库概率≈0，端到端无法稳定复现；
// 冲突防护位于存储层条件写（local.go 的 O_CREATE|O_EXCL），需要真实预占路径才能验证，
// 与云驱动的条件写同属已记录的测试债。
func TestLocalUploadSameNameTwiceGetsDistinctNames(t *testing.T) {
	storage, _ := setupLocalUpload(t)
	ctx := context.Background()

	first, err := storage.Upload(ctx, pngFileHeader(t, "微信截图.png"))
	if err != nil {
		t.Fatalf("第一次 Upload: %v", err)
	}
	// Upload 会消费 fileHeader，第二次必须重新构造一份
	second, err := storage.Upload(ctx, pngFileHeader(t, "微信截图.png"))
	if err != nil {
		t.Fatalf("第二次 Upload: %v", err)
	}

	if first.Path == second.Path {
		t.Errorf("两次上传路径相同 %q, 随机命名未生效", first.Path)
	}
	for _, att := range []*dto.AttachmentDTO{first, second} {
		if att.Name != "微信截图" {
			t.Errorf("Name = %q, want 两次都保留原始基名 %q", att.Name, "微信截图")
		}
	}
}
