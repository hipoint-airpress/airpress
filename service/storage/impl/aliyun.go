package filestorageimpl

import (
	"context"
	"image"
	"io"
	"mime/multipart"
	"net/url"
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
	Bucket         *oss.Bucket
	BucketName     string
	EndPoint       string
	Source         string
	Style          string
	ThumbnailStyle string
	Domain         string
	Protocol       string
}

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
		if aliyunClientInstance.ThumbnailStyle != "" {
			return fd.getRelativePath() + aliyunClientInstance.ThumbnailStyle, nil
		} else {
			return fd.getRelativePath(), nil
		}
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
	aliyunClientInstance.ThumbnailStyle = thumbnailStyleRule
	aliyunClientInstance.Style = styleRule
	aliyunClientInstance.Domain = domain
	aliyunClientInstance.Protocol = protocol
	aliyunClientInstance.Bucket, err = client.Bucket(bucketName)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusInternalServerError).WithMsg("failed to initialize aliyun oss client bucket: " + err.Error())
	}
	return aliyunClientInstance, nil
}
