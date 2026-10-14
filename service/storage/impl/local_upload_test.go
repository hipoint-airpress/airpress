package filestorageimpl

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/config"
	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
)

// 本文件端到端验证 LocalFileStorage.Upload：真实 multipart、真实 gorm+内存 SQLite、真实落盘。
// 注意 Upload 自身只通过 commonRenamePredicateFunc 读库查重；附件行写库发生在服务层
// attachmentServiceImpl.Upload，这里按服务层同样的转换补一次 Create，再读回比对。
// 禁止 t.Parallel()：dal 包级全局 Q 是共享状态。

// uploadPathPattern 钉住落盘契约：upload/{类型}/{年}/{月}/{10 位 base62 随机名}.png
var uploadPathPattern = regexp.MustCompile(`^upload/img/[0-9]{4}/[0-9]{2}/[0-9A-Za-z]{10}\.png$`)

// memoryDSNSeq 给每个测试一个独立的共享内存库名，避免先后两个测试间残留数据
var memoryDSNSeq int64

// setupLocalUpload 用内存 SQLite 初始化全局 dal.Q，并返回以 t.TempDir 为 WorkDir 的本地存储。
// Q 的旧值与连接关闭都在 t.Cleanup 里恢复，保证不污染同包其它测试、也不使用仓库的 airpress.db / upload/。
func setupLocalUpload(t *testing.T) (*LocalFileStorage, string) {
	t.Helper()

	dsn := fmt.Sprintf("file:airpress_local_upload_%d?mode=memory&cache=shared",
		atomic.AddInt64(&memoryDSNSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	// :memory: 库每连接一份，连接数压到 1 配合共享缓存 DSN，保证所有查询命中同一个库
	sqlDB.SetMaxOpenConns(1)
	// 只迁移本测试需要的表；不碰仓库的 airpress.db，也不跑全量 dbMigrate
	if err := db.AutoMigrate(&entity.Attachment{}); err != nil {
		t.Fatalf("auto migrate attachment: %v", err)
	}

	// Upload 内 GetQueryByCtx(ctx) 在 ctx 无事务时回落到包级全局 Q，先存旧值、测试结束恢复
	prev := *dal.Q
	t.Cleanup(func() {
		*dal.Q = prev
		_ = sqlDB.Close()
	})
	dal.SetDefault(db)

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

// saveAttachment 复刻服务层的 DTO→entity 转换写一行附件，用于验证真实数据库写入路径。
// 与服务层的差异只在 ThumbPath 取 DTO.ThumbPath（服务层写法见其源码，疑似 bug，本测试不照搬）。
func saveAttachment(t *testing.T, ctx context.Context, att *dto.AttachmentDTO) {
	t.Helper()

	sep := string(os.PathSeparator)
	rec := &entity.Attachment{
		FileKey:   att.FileKey,
		Height:    att.Height,
		MediaType: att.MediaType,
		Name:      att.Name,
		Path:      strings.ReplaceAll(att.Path, sep, "/"),
		Size:      att.Size,
		Suffix:    att.Suffix,
		ThumbPath: strings.ReplaceAll(att.ThumbPath, sep, "/"),
		Type:      att.AttachmentType,
		Width:     att.Width,
	}
	if err := dal.Q.Attachment.WithContext(ctx).Create(rec); err != nil {
		t.Fatalf("create attachment: %v", err)
	}
}

// 场景一：中文命名 PNG 上传成功并正确落盘、正确写库
func TestLocalUploadChinesePngSavesAndPersists(t *testing.T) {
	storage, workDir := setupLocalUpload(t)
	ctx := context.Background()

	att, err := storage.Upload(ctx, pngFileHeader(t, "微信截图.png"))
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if att.Name != "微信截图" {
		t.Errorf("Name = %q, want 原始基名 %q", att.Name, "微信截图")
	}
	if !uploadPathPattern.MatchString(att.Path) {
		t.Errorf("Path = %q, 不符合落盘契约 %q", att.Path, uploadPathPattern.String())
	}
	if att.FileKey != att.Path {
		t.Errorf("FileKey = %q, want 与 Path 相同 %q", att.FileKey, att.Path)
	}
	if !strings.Contains(att.MediaType, "image/png") {
		t.Errorf("MediaType = %q, want 含 image/png", att.MediaType)
	}
	if att.Width != 2 || att.Height != 2 {
		t.Errorf("Width/Height = %d/%d, want 2/2", att.Width, att.Height)
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

	// 真实数据库写入后读回比对
	saveAttachment(t, ctx, att)
	rec, err := dal.Q.Attachment.WithContext(ctx).
		Where(dal.Q.Attachment.Path.Eq(att.Path)).First()
	if err != nil {
		t.Fatalf("读回附件失败: %v", err)
	}
	if rec.Name != att.Name || rec.Path != att.Path || rec.FileKey != att.FileKey {
		t.Errorf("库内行与 DTO 不一致: name=%q path=%q file_key=%q, want name=%q path=%q file_key=%q",
			rec.Name, rec.Path, rec.FileKey, att.Name, att.Path, att.FileKey)
	}
}

// 场景二：同名二次上传得到不同随机名，两条记录共存
func TestLocalUploadSameNameTwiceGetsDistinctNames(t *testing.T) {
	storage, _ := setupLocalUpload(t)
	ctx := context.Background()

	first, err := storage.Upload(ctx, pngFileHeader(t, "微信截图.png"))
	if err != nil {
		t.Fatalf("第一次 Upload: %v", err)
	}
	saveAttachment(t, ctx, first)

	// Upload 会消费 fileHeader，第二次必须重新构造一份
	second, err := storage.Upload(ctx, pngFileHeader(t, "微信截图.png"))
	if err != nil {
		t.Fatalf("第二次 Upload: %v", err)
	}
	saveAttachment(t, ctx, second)

	if first.Path == second.Path {
		t.Errorf("两次上传路径相同 %q, 随机命名未生效", first.Path)
	}
	for _, att := range []*dto.AttachmentDTO{first, second} {
		if att.Name != "微信截图" {
			t.Errorf("Name = %q, want 两次都保留原始基名 %q", att.Name, "微信截图")
		}
	}

	count, err := dal.Q.Attachment.WithContext(ctx).Count()
	if err != nil {
		t.Fatalf("count attachments: %v", err)
	}
	if count != 2 {
		t.Errorf("附件表行数 = %d, want 2（两条记录共存）", count)
	}
}
