package filestorageimpl

import (
	"context"
	"image"
	"io"
	"mime/multipart"
	"net/url"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type Aliyun struct {
	OptionService service.OptionService
}

func NewAliyun(optionService service.OptionService) *Aliyun {
	return &Aliyun{
		OptionService: optionService,
	}
}

type aliyunClient struct {
	*oss.Client
	Bucket     *oss.Bucket
	BucketName string
	EndPoint   string
	Source     string
	// Style 是主图的 OSS 处理参数查询串（已含 ?x-oss-process= 前缀），空串表示不处理
	Style string
	// ThumbnailStyle 是缩略图的 OSS 处理参数查询串（已含 ?x-oss-process= 前缀），空串表示不处理
	ThumbnailStyle string
	Domain         string
	Protocol       string
}

const (
	// ossProcessPrefix 是 OSS 图片处理参数的查询串前缀，直接追加在对象 key 之后
	ossProcessPrefix = "?x-oss-process="
	// ossStylePrefix 表示配置值是 OSS 控制台里创建的样式名
	ossStylePrefix = "style/"
)

func (a *Aliyun) Upload(ctx context.Context, fileHeader *multipart.FileHeader) (*dto.AttachmentDTO, error) {
	aliyunClientInstance, err := a.getAliOSSClient(ctx)
	if err != nil {
		return nil, err
	}

	basePath := aliyunClientInstance.Protocol + aliyunClientInstance.BucketName + "." + aliyunClientInstance.EndPoint
	if aliyunClientInstance.Domain != "" {
		basePath = aliyunClientInstance.Protocol + aliyunClientInstance.Domain
	}
	fd, err := newURLFileDescriptor(
		withBaseURL(basePath),
		withSubURLPath(cloudSubPath(aliyunClientInstance.Source, time.Now(), fileHeader.Filename)),
		withShouldRenameURLOption(commonRenamePredicateFunc(ctx, consts.AttachmentTypeAliOSS)),
		withOriginalNameURLOption(fileHeader.Filename),
		withRenameURLOption(storageName),
	)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusInternalServerError)
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusInternalServerError).WithMsg("open upload file error")
	}
	defer func() { _ = file.Close() }()
	// 原始文件名写进对象元数据，此后直链访问由 OSS 直接返回该头，无需查库
	err = aliyunClientInstance.Bucket.PutObject(
		fd.getRelativePath(),
		file,
		oss.ContentDisposition(contentDisposition(fileHeader.Filename)),
		// 系统语义为「不允许覆盖」：同名对象已存在时由 OSS 直接拒绝，而不是静默替换旧附件内容
		oss.ForbidOverWrite(true),
	)
	if err != nil {
		return nil, xerr.WithMsg(err, "upload to aliyun oss error: "+err.Error()).WithStatus(xerr.StatusInternalServerError)
	}
	mediaType, _ := getFileContentType(file)
	result := &dto.AttachmentDTO{
		Name:           uploadBaseName(fileHeader.Filename),
		Path:           fd.getRelativePath() + aliyunClientInstance.Style,
		FileKey:        fd.getRelativePath(),
		Suffix:         fd.getExtensionName(),
		MediaType:      mediaType,
		AttachmentType: consts.AttachmentTypeAliOSS,
		Size:           fileHeader.Size,
	}
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusInternalServerError)
	}
	err = handleImageMeta(file, result, func(_ image.Image) (string, error) {
		return fd.getRelativePath() + aliyunClientInstance.ThumbnailStyle, nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a *Aliyun) Delete(ctx context.Context, fileKey string) error {
	aliyunClientInstance, err := a.getAliOSSClient(ctx)
	if err != nil {
		return err
	}
	err = aliyunClientInstance.Bucket.DeleteObject(fileKey)
	if err != nil {
		return xerr.WithMsg(err, "delete file err from aliyun oss").WithStatus(xerr.StatusInternalServerError)
	}
	return nil
}

func (a *Aliyun) GetAttachmentType() consts.AttachmentType {
	return consts.AttachmentTypeAliOSS
}

func (a *Aliyun) GetFilePath(ctx context.Context, relativePath string) (string, error) {
	aliyunClientInstance, err := a.getAliOSSClient(ctx)
	if err != nil {
		return "", err
	}

	basePath := aliyunClientInstance.Protocol + aliyunClientInstance.BucketName + "." + aliyunClientInstance.EndPoint
	if aliyunClientInstance.Domain != "" {
		basePath = aliyunClientInstance.Protocol + aliyunClientInstance.Domain
	}
	fullPath, _ := url.JoinPath(basePath, relativePath)
	fullPath, _ = url.PathUnescape(fullPath)
	return fullPath, nil
}

func (a *Aliyun) getAliOSSClient(ctx context.Context) (*aliyunClient, error) {
	getClientProperty := func(propertyValue *string, property property.Property, e error) error {
		if e != nil {
			return e
		}
		value, err := a.OptionService.GetOrByDefaultWithErr(ctx, property, property.DefaultValue)
		if err != nil {
			return err
		}
		strValue, ok := value.(string)
		if !ok {
			return xerr.WithStatus(nil, xerr.StatusBadRequest).WithErrMsgf("wrong property type")
		}
		*propertyValue = strValue
		return nil
	}
	var endPoint, bucketName, accessKey, accessSecret, source, styleRule, thumbnailStyleRule, domain, protocol string
	err := getClientProperty(&endPoint, property.AliOssEndpoint, nil)
	err = getClientProperty(&bucketName, property.AliOssBucketName, err)
	err = getClientProperty(&accessKey, property.AliOssAccessKey, err)
	err = getClientProperty(&accessSecret, property.AliOssAccessSecret, err)
	err = getClientProperty(&source, property.AliOssSource, err)
	err = getClientProperty(&styleRule, property.AliOssStyleRule, err)
	err = getClientProperty(&thumbnailStyleRule, property.AliOssThumbnailStyleRule, err)
	err = getClientProperty(&domain, property.AliOssDomain, err)
	err = getClientProperty(&protocol, property.AliOssProtocol, err)
	if err != nil {
		return nil, err
	}
	client, err := oss.New(endPoint, accessKey, accessSecret, oss.Timeout(2, 60), oss.EnableCRC(true))
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusInternalServerError).WithMsg("failed to initialize aliyun oss client: " + err.Error())
	}
	aliyunClientInstance := &aliyunClient{}
	aliyunClientInstance.Client = client
	aliyunClientInstance.Source = source
	aliyunClientInstance.BucketName = bucketName
	aliyunClientInstance.EndPoint = endPoint
	// 存解析后的最终查询串，使用点直接拼接、不再关心配置形态
	aliyunClientInstance.ThumbnailStyle = ossProcessQuery(thumbnailStyleRule)
	aliyunClientInstance.Style = ossProcessQuery(styleRule)
	aliyunClientInstance.Domain = domain
	aliyunClientInstance.Protocol = protocol
	aliyunClientInstance.Bucket, err = client.Bucket(bucketName)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusInternalServerError).WithMsg("failed to initialize aliyun oss client bucket: " + err.Error())
	}
	return aliyunClientInstance, nil
}

// ossProcessQuery 把配置的样式规则解析成可追加到对象 key 之后的查询串：
// 含 "/" 视为完整处理表达式（image/resize,w_256），不含 "/" 视为样式名（thumb → style/thumb），
// 空值返回空串表示不做任何处理。
func ossProcessQuery(rule string) string {
	trimmed := strings.TrimSpace(rule)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "/") {
		return ossProcessPrefix + trimmed
	}
	return ossProcessPrefix + ossStylePrefix + trimmed
}
