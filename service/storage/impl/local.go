package filestorageimpl

import (
	"context"
	"image"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/disintegration/imaging"
	"go.uber.org/zap"
	_ "golang.org/x/image/webp"

	"github.com/hipoint-airpress/airpress/config"
	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/log"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

var (
	thumbWidth      = 256
	thumbHeight     = 256
	thumbnailSuffix = "-thumbnail"
)

type LocalFileStorage struct {
	Config        *config.Config
	OptionService service.OptionService
}

func NewLocalFileStorage(config *config.Config, optionService service.OptionService) *LocalFileStorage {
	return &LocalFileStorage{
		Config:        config,
		OptionService: optionService,
	}
}

func (l *LocalFileStorage) Upload(ctx context.Context, fileHeader *multipart.FileHeader) (*dto.AttachmentDTO, error) {
	now := time.Now()
	subPath := l.getUploadPath(now, fileHeader.Filename)

	fd, err := newLocalFileDescriptor(
		withOriginalName(fileHeader.Filename),
		withBasePath(l.Config.AirPress.WorkDir),
		withSubPath(subPath),
		withRename(storageName),
	)
	if err != nil {
		return nil, err
	}

	log.CtxDebug(ctx, "Local FileStorage ",
		zap.String("originalFilename", fileHeader.Filename),
		zap.String("absPath", fd.getFullPath()))

	err = os.MkdirAll(fd.getFullDirPath(), os.ModePerm)
	if isExist := os.IsExist(err); err != nil && !isExist {
		return nil, xerr.WithMsg(err, "Upload file error")
	}

	srcFile, err := fileHeader.Open()
	if err != nil {
		return nil, xerr.WithMsg(err, "Upload file error")
	}
	defer func() { _ = srcFile.Close() }()

	// O_EXCL：路径已存在则失败，避免静默截断覆盖既有附件（os.Create 是 O_CREATE|O_TRUNC，永不返回 ErrExist）
	out, err := os.OpenFile(fd.getFullPath(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		return nil, xerr.WithMsg(err, "The file already exists!")
	} else if err != nil {
		return nil, xerr.WithMsg(err, "Upload file error")
	}
	defer func() { _ = out.Close() }()
	defer func() {
		if err != nil {
			_ = l.Delete(ctx, fd.getFullPath())
		}
	}()

	_, err = io.Copy(out, srcFile)
	if err != nil {
		return nil, xerr.WithMsg(err, "Error writing file")
	}

	mediaType, _ := getFileContentType(srcFile)
	attachment := dto.AttachmentDTO{
		Name:           uploadBaseName(fileHeader.Filename),
		Path:           fd.getRelativePath(),
		FileKey:        fd.getRelativePath(),
		Suffix:         fd.getExtensionName(),
		MediaType:      mediaType,
		AttachmentType: consts.AttachmentTypeLocal,
		Size:           fileHeader.Size,
	}

	thumbnailFn := func(srcImage image.Image) (string, error) {
		if fd.getExtensionName() == "webp" {
			return fd.getRelativePath(), nil
		}
		// 缩略图名派生自主图 fd 的名字：回调发生在 handleImageMeta 内、主图 fd 构造完成之后，
		// 因此 getFileName() 取到的是主图最终采用的名字，保证 Delete 按「主图基名 + -thumbnail」
		// 推导（见本文件 Delete）始终命中。不要退回成独立生成随机名；subPath 复用主图那份，保证同目录。
		thumbnailFd, err := newLocalFileDescriptor(
			withOriginalName(fileHeader.Filename),
			withBasePath(l.Config.AirPress.WorkDir),
			withSubPath(subPath),
			withRename(func() (string, error) { return fd.getFileName(), nil }),
			withSuffix("-thumbnail"),
		)
		if err != nil {
			return "", err
		}
		bounds := srcImage.Bounds()
		srcWidth := bounds.Dx()
		srcHeight := bounds.Dy()
		width, height := keepAspectRatio(srcWidth, srcHeight, thumbWidth, thumbHeight)

		dstImage := imaging.Thumbnail(srcImage, width, height, imaging.Box)
		err = imaging.Save(dstImage, thumbnailFd.getFullPath())
		if err != nil {
			return "", xerr.NoType.Wrap(err).WithMsg("Save thumb srcImage err")
		}
		return thumbnailFd.getRelativePath(), nil
	}
	_, err = srcFile.Seek(0, io.SeekStart)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusInternalServerError)
	}
	err = handleImageMeta(srcFile, &attachment, thumbnailFn)
	if err != nil {
		return nil, err
	}
	return &attachment, nil
}

func (l *LocalFileStorage) Delete(ctx context.Context, fileKey string) error {
	filePath, err := filepath.Abs(fileKey)
	if err != nil {
		return xerr.NoType.Wrap(err)
	}
	extName := filepath.Ext(fileKey)
	fullName := path.Base(fileKey)
	originalFilename := fullName[0 : len(fullName)-len(extName)]

	thumbFileName := originalFilename + thumbnailSuffix + extName
	thumbFilePath := filepath.Join(filepath.Dir(fileKey), thumbFileName)

	err = os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return xerr.NoType.Wrap(err).WithMsg("delete file failed")
	}
	err = os.Remove(thumbFilePath)
	if err != nil && !os.IsNotExist(err) {
		return xerr.NoType.Wrap(err).WithMsg("delete file failed")
	}
	return nil
}

func (l *LocalFileStorage) GetAttachmentType() consts.AttachmentType {
	return consts.AttachmentTypeLocal
}

func (l *LocalFileStorage) GetFilePath(ctx context.Context, relativePath string) (string, error) {
	isEnabled, _ := l.OptionService.IsEnabledAbsolutePath(ctx)
	var blogBaseURL string
	if isEnabled {
		blogBaseURL, _ = l.OptionService.GetBlogBaseURL(ctx)
	}
	fullPath, _ := url.JoinPath(blogBaseURL, relativePath)
	if blogBaseURL == "" {
		fullPath, _ = url.JoinPath("/", relativePath)
	}
	fullPath, _ = url.PathUnescape(fullPath)
	return fullPath, nil
}

// getUploadPath 返回 upload/{类型}/{年}/{月} 形式的本地存储目录
func (l *LocalFileStorage) getUploadPath(t time.Time, filename string) string {
	return filepath.Clean(filepath.Join(
		consts.AirPressUploadDir,
		filepath.FromSlash(storageSubPath(t, filename)),
	))
}
