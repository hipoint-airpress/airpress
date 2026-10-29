package impl

import (
	"context"
	"fmt"
	"mime/multipart"
	"strings"
	"sync/atomic"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/service/storage"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

// 本文件对 attachmentServiceImpl.Upload 与读取侧 ConvertToDTO(s) 做集成级测试:
// 真实 gorm + 内存 SQLite 写库读回,存储层用桩替换,钉死两条已修行为:
//  1. thumb_path 入库等于 DTO.ThumbPath(守卫 4e348dc 修掉的「取错字段导致恒等于主图路径」缺陷);
//  2. 空 ThumbPath 不再被喂给 GetFilePath(守卫「非图片附件的 thumbPath 吐出存储桶根地址」的短路修复)。
//
// 禁止 t.Parallel():dal 包级全局 Q 是共享状态。

// attachmentUploadDSNSeq 给每个测试一个独立的共享内存库名,避免先后两个测试间残留数据
var attachmentUploadDSNSeq int64

// stubFileStorage 实现 storage.FileStorage:
// Upload 每次返回 uploadDTO 的一份独立副本(服务层会在返回的 DTO 上改写 ID/Path/ThumbPath,
// 不能让它污染桩配置,重复路径用例因此每次拿到的都是同样的原始值);
// GetFilePath 返回可辨识的 CDN 地址并记录全部入参,用于断言空串根本没被喂进来。
// 桩刻意不返回错误:ConvertToDTOs 的失败分支会走 log.CtxError(默认 zap.NewNop,不会 panic,
// 但本测试不关心该分支,让它不可达)。
type stubFileStorage struct {
	uploadDTO     *dto.AttachmentDTO // Upload 桩返回的原始 DTO,每次调用拷贝一份
	filePathCalls []string           // GetFilePath 收到的 relativePath,按调用顺序记录
}

func (s *stubFileStorage) Upload(ctx context.Context, file *multipart.FileHeader) (*dto.AttachmentDTO, error) {
	if s.uploadDTO == nil {
		return nil, fmt.Errorf("stubFileStorage: 未配置 uploadDTO")
	}
	copied := *s.uploadDTO
	return &copied, nil
}

func (s *stubFileStorage) Delete(ctx context.Context, fileKey string) error { return nil }

func (s *stubFileStorage) GetAttachmentType() consts.AttachmentType {
	return consts.AttachmentTypeLocal
}

func (s *stubFileStorage) GetFilePath(ctx context.Context, relativePath string) (string, error) {
	s.filePathCalls = append(s.filePathCalls, relativePath)
	return "https://cdn.test/" + relativePath, nil
}

// stubStorageComposite 实现 storage.FileStorageComposite:不分存储类型,恒返回同一个桩
type stubStorageComposite struct {
	storage storage.FileStorage
}

func (c *stubStorageComposite) GetFileStorage(consts.AttachmentType) storage.FileStorage {
	return c.storage
}

// stubOptionService 只实现附件服务会用到的 GetAttachmentType,
// 其余方法经接口嵌入保持不可达(被调用即对 nil 接口分派而 panic,等于隐式断言不走到)
type stubOptionService struct {
	service.OptionService
	attachmentType consts.AttachmentType
}

func (s *stubOptionService) GetAttachmentType(ctx context.Context) consts.AttachmentType {
	return s.attachmentType
}

// stubUserService 只实现归属人查询，返回空列表。
type stubUserService struct {
	service.UserService
}

func (stubUserService) GetByIDs(ctx context.Context, ids []int32) ([]*entity.User, error) {
	return make([]*entity.User, 0), nil
}

// setupAttachmentUpload 用内存 SQLite 初始化全局 dal.Q,并按生产装配(NewAttachmentService)
// 接线存储桩,返回服务与桩。做法与 service/storage/impl/local_upload_test.go 的 setupLocalUpload
// 一致:共享内存 DSN、连接数压到 1、只迁移 attachment 表、备份全局 Q 并在 t.Cleanup 恢复。
func setupAttachmentUpload(t *testing.T) (service.AttachmentService, *stubFileStorage) {
	t.Helper()

	dsn := fmt.Sprintf("file:airpress_attachment_upload_%d?mode=memory&cache=shared",
		atomic.AddInt64(&attachmentUploadDSNSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	// :memory: 库每连接一份,连接数压到 1 配合共享缓存 DSN,保证所有查询命中同一个库
	sqlDB.SetMaxOpenConns(1)
	// 只迁移本测试需要的表
	if err := db.AutoMigrate(&entity.Attachment{}); err != nil {
		t.Fatalf("auto migrate attachment: %v", err)
	}
	// Upload 内 GetQueryByCtx(ctx) 在 ctx 无事务时回落到包级全局 Q,先存旧值、测试结束恢复。
	// 恢复依赖 SetDefault 的原地改写形态(dal/gen.go:38),同 local_upload_test.go 的说明。
	prev := *dal.Q
	t.Cleanup(func() {
		*dal.Q = prev
		_ = sqlDB.Close()
	})
	dal.SetDefault(db)

	stub := &stubFileStorage{}
	svc := NewAttachmentService(
		&stubOptionService{attachmentType: consts.AttachmentTypeLocal},
		&stubStorageComposite{storage: stub},
		stubUserService{},
	)
	return svc, stub
}

// findAttachment 按 path 读回库里的唯一一行
func findAttachment(t *testing.T, ctx context.Context, path string) *entity.Attachment {
	t.Helper()
	rec, err := dal.Q.Attachment.WithContext(ctx).Where(dal.Q.Attachment.Path.Eq(path)).First()
	if err != nil {
		t.Fatalf("按 path=%q 读回附件失败: %v", path, err)
	}
	return rec
}

// calledWith 判断 GetFilePath 的入参记录里是否出现过 want
func calledWith(calls []string, want string) bool {
	for _, c := range calls {
		if c == want {
			return true
		}
	}
	return false
}

// imageDTO 构造一个图片附件的桩返回值
func imageDTO(path, thumbPath string) *dto.AttachmentDTO {
	return &dto.AttachmentDTO{
		Name:           "a",
		Path:           path,
		FileKey:        path,
		ThumbPath:      thumbPath,
		MediaType:      "image/png",
		Suffix:         "png",
		Width:          2,
		Height:         2,
		Size:           123,
		AttachmentType: consts.AttachmentTypeLocal,
	}
}

// nonImageDTO 构造一个非图片附件(pdf)的桩返回值:三个真实驱动此时都不给 ThumbPath
func nonImageDTO(path string) *dto.AttachmentDTO {
	return &dto.AttachmentDTO{
		Name:           "doc",
		Path:           path,
		FileKey:        path,
		ThumbPath:      "",
		MediaType:      "application/pdf",
		Suffix:         "pdf",
		Width:          0,
		Height:         0,
		Size:           456,
		AttachmentType: consts.AttachmentTypeLocal,
	}
}

// 场景一:图片附件上传后,库里 thumb_path 等于 DTO.ThumbPath 且不等于 path。
// 这是防「service/impl/attachment.go 的 entity.ThumbPath 退回 attachmentDTO.Path」的守卫。
func TestUploadImagePersistsThumbPathDistinctFromPath(t *testing.T) {
	svc, stub := setupAttachmentUpload(t)
	ctx := context.Background()

	const wantPath = "upload/img/2026/10/a.png"
	const wantThumbPath = "upload/img/2026/10/a-thumbnail.png"
	stub.uploadDTO = imageDTO(wantPath, wantThumbPath)

	// 桩不消费 fileHeader,服务层也只把它透传给存储,传 nil 即可
	if _, err := svc.Upload(ctx, nil); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	rec := findAttachment(t, ctx, wantPath)
	if rec.Path != wantPath {
		t.Errorf("path = %q, want %q", rec.Path, wantPath)
	}
	if rec.ThumbPath != wantThumbPath {
		t.Errorf("thumb_path = %q, want DTO.ThumbPath %q", rec.ThumbPath, wantThumbPath)
	}
	if rec.ThumbPath == rec.Path {
		t.Errorf("thumb_path 与 path 相同(%q),thumb_path 入库取错字段的缺陷(4e348dc)回归", rec.ThumbPath)
	}

	// 逐列守住 DTO→entity 的真实映射（attachment.go 的建实体段）：
	// 漏写、错位、Width/Height 互换、文件名取成随机名等回归都要在这里变红。
	// 原先这几列只由存储层测试手抄的映射覆盖，搬到这里后测的是真实代码。
	if rec.Name != "a" {
		t.Errorf("name = %q, want %q", rec.Name, "a")
	}
	if rec.FileKey != wantPath {
		t.Errorf("file_key = %q, want %q", rec.FileKey, wantPath)
	}
	if rec.Suffix != "png" {
		t.Errorf("suffix = %q, want %q", rec.Suffix, "png")
	}
	if rec.MediaType != "image/png" {
		t.Errorf("media_type = %q, want %q", rec.MediaType, "image/png")
	}
	if rec.Size != 123 {
		t.Errorf("size = %d, want 123", rec.Size)
	}
	if rec.Width != 2 || rec.Height != 2 {
		t.Errorf("width/height = %d/%d, want 2/2", rec.Width, rec.Height)
	}
	if rec.Type != consts.AttachmentTypeLocal {
		t.Errorf("type = %v, want %v", rec.Type, consts.AttachmentTypeLocal)
	}
}

// 场景二:非图片附件上传后,库里 thumb_path 为空串——既不是主图路径,也不是任何 URL。
func TestUploadNonImagePersistsEmptyThumbPath(t *testing.T) {
	svc, stub := setupAttachmentUpload(t)
	ctx := context.Background()

	const wantPath = "upload/attachments/2026/10/doc.pdf"
	stub.uploadDTO = nonImageDTO(wantPath)

	att, err := svc.Upload(ctx, nil)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	rec := findAttachment(t, ctx, wantPath)
	if rec.Path != wantPath {
		t.Errorf("path = %q, want %q", rec.Path, wantPath)
	}
	if rec.ThumbPath != "" {
		t.Errorf("thumb_path = %q, want 空串(非图片附件没有缩略图,不得等于主图路径 %q)", rec.ThumbPath, wantPath)
	}
	// 服务层返回值同样短路:空 ThumbPath 不应被解析成任何 URL(含存储桶根地址)
	if att.ThumbPath != "" {
		t.Errorf("返回 DTO.ThumbPath = %q, want 空串", att.ThumbPath)
	}
	if calledWith(stub.filePathCalls, "") {
		t.Errorf("GetFilePath 收到过空串入参,空缩略图短路失效: calls=%q", strings.Join(stub.filePathCalls, ", "))
	}
}

// 场景三:读取侧短路。thumb_path 为空的行走 ConvertToDTO / ConvertToDTOs,
// 返回的 dto.ThumbPath 为空串、GetFilePath 没收到过空串入参;
// 同时 dto.Path 仍被正常解析(短路只影响缩略图,没有波及主图路径)。
func TestConvertToDTOSkipsEmptyThumbPath(t *testing.T) {
	svc, stub := setupAttachmentUpload(t)
	ctx := context.Background()

	const wantPath = "upload/attachments/2026/10/doc.pdf"
	stub.uploadDTO = nonImageDTO(wantPath)
	// 用真实写入路径产出 thumb_path="" 的行(同场景二)
	if _, err := svc.Upload(ctx, nil); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	// 只观察读取侧的调用
	stub.filePathCalls = nil

	rec := findAttachment(t, ctx, wantPath)

	single, err := svc.ConvertToDTO(ctx, rec)
	if err != nil {
		t.Fatalf("ConvertToDTO: %v", err)
	}
	if single.ThumbPath != "" {
		t.Errorf("ConvertToDTO 的 thumbPath = %q, want 空串", single.ThumbPath)
	}
	if single.Path != "https://cdn.test/"+wantPath {
		t.Errorf("ConvertToDTO 的 path = %q, want 主图被正常解析为 %q", single.Path, "https://cdn.test/"+wantPath)
	}

	batch, err := svc.ConvertToDTOs(ctx, []*entity.Attachment{rec})
	if err != nil {
		t.Fatalf("ConvertToDTOs: %v", err)
	}
	if len(batch) != 1 {
		t.Fatalf("ConvertToDTOs 返回 %d 条, want 1", len(batch))
	}
	if batch[0].ThumbPath != "" {
		t.Errorf("ConvertToDTOs 的 thumbPath = %q, want 空串", batch[0].ThumbPath)
	}
	if batch[0].Path != "https://cdn.test/"+wantPath {
		t.Errorf("ConvertToDTOs 的 path = %q, want 主图被正常解析为 %q", batch[0].Path, "https://cdn.test/"+wantPath)
	}

	if calledWith(stub.filePathCalls, "") {
		t.Errorf("GetFilePath 收到过空串入参,空缩略图短路失效: calls=%q", strings.Join(stub.filePathCalls, ", "))
	}
	if !calledWith(stub.filePathCalls, wantPath) {
		t.Errorf("GetFilePath 未收到主图路径入参 %q, 主图解析被误伤: calls=%q", wantPath, strings.Join(stub.filePathCalls, ", "))
	}
}

// 场景四:桩两次返回同一个 Path,第二次必须命中服务层查重并报 400——
// 证明集成测试没有绕过 Upload 开头的既有校验。
func TestUploadDuplicatePathRejected(t *testing.T) {
	svc, stub := setupAttachmentUpload(t)
	ctx := context.Background()

	const wantPath = "upload/img/2026/10/dup.png"
	stub.uploadDTO = imageDTO(wantPath, "upload/img/2026/10/dup-thumbnail.png")
	if _, err := svc.Upload(ctx, nil); err != nil {
		t.Fatalf("第一次 Upload: %v", err)
	}

	if _, err := svc.Upload(ctx, nil); err == nil {
		t.Fatalf("第二次 Upload 应与第一次同路径,期望服务层查重报 400,实际 err = nil")
	} else {
		if status := xerr.GetHTTPStatus(err); status != xerr.StatusBadRequest {
			t.Errorf("重复路径错误的 HTTP 状态 = %d, want %d", status, xerr.StatusBadRequest)
		}
		if !strings.Contains(err.Error(), "已经存在") {
			t.Errorf("重复路径错误信息 = %q, want 含「已经存在」", err.Error())
		}
	}

	count, err := dal.Q.Attachment.WithContext(ctx).Count()
	if err != nil {
		t.Fatalf("count attachments: %v", err)
	}
	if count != 1 {
		t.Errorf("附件表行数 = %d, want 1(第二次上传应被拒绝,不得写库)", count)
	}
}
