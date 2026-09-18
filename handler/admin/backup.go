package admin

import (
	"errors"
	"net/http"
	"path"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/hipoint-airpress/airpress/config"
	"github.com/hipoint-airpress/airpress/handler/trans"
	"github.com/hipoint-airpress/airpress/log"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type BackupHandler struct {
	BackupService service.BackupService
}

func NewBackupHandler(backupService service.BackupService) *BackupHandler {
	return &BackupHandler{
		BackupService: backupService,
	}
}

// GetWorkDirBackup godoc
// @Summary      获取工作目录备份信息
// @Description  根据 filename 查询工作目录备份的元信息
// @Tags         Admin.Backup
// @Produce      json
// @Param        filename  query     string  true  "备份文件名"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.BackupDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/backups/work-dir [get]
// @Description  此方法为 HandleWorkDir 内部分发器调用的子方法,通过 fetch 查询参数区分
func (b *BackupHandler) GetWorkDirBackup(ctx *gin.Context) (interface{}, error) {
	filename, err := util.MustGetQueryString(ctx, "filename")
	if err != nil {
		return nil, err
	}
	return b.BackupService.GetBackup(ctx, filepath.Join(config.BackupDir, filename), service.WholeSite)
}

// GetDataBackup godoc
// @Summary      获取数据备份信息
// @Description  根据 filename 查询数据备份的元信息
// @Tags         Admin.Backup
// @Produce      json
// @Param        filename  query     string  true  "备份文件名"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.BackupDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/backups/data [get]
// @Description  此方法为 HandleData 内部分发器调用的子方法,通过 fetch 查询参数区分
func (b *BackupHandler) GetDataBackup(ctx *gin.Context) (interface{}, error) {
	filename, err := util.MustGetQueryString(ctx, "filename")
	if err != nil {
		return nil, err
	}
	return b.BackupService.GetBackup(ctx, filepath.Join(config.DataExportDir, filename), service.JSONData)
}

// GetMarkDownBackup godoc
// @Summary      获取 Markdown 备份信息
// @Description  根据 filename 查询 Markdown 备份的元信息
// @Tags         Admin.Backup
// @Produce      json
// @Param        filename  query     string  true  "备份文件名"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.BackupDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/backups/markdown/fetch [get]
func (b *BackupHandler) GetMarkDownBackup(ctx *gin.Context) (interface{}, error) {
	filename, err := util.MustGetQueryString(ctx, "filename")
	if err != nil {
		return nil, err
	}
	return b.BackupService.GetBackup(ctx, filepath.Join(config.BackupMarkdownDir, filename), service.Markdown)
}

// BackupWholeSite godoc
// @Summary      备份整个站点工作目录
// @Description  根据传入的待备份项列表,打包工作目录为备份文件
// @Tags         Admin.Backup
// @Accept       json
// @Produce      json
// @Param        toBackupItems  body     []string  true  "待备份项列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.BackupDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/backups/work-dir [post]
func (b *BackupHandler) BackupWholeSite(ctx *gin.Context) (interface{}, error) {
	toBackupItems := make([]string, 0)
	err := ctx.ShouldBindJSON(&toBackupItems)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest)
	}

	return b.BackupService.BackupWholeSite(ctx, toBackupItems)
}

// ListBackups godoc
// @Summary      列出工作目录备份
// @Description  返回工作目录下所有备份文件的元信息
// @Tags         Admin.Backup
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.BackupDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/backups/work-dir [get]
func (b *BackupHandler) ListBackups(ctx *gin.Context) (interface{}, error) {
	return b.BackupService.ListFiles(ctx, config.BackupDir, service.WholeSite)
}

// ListToBackupItems godoc
// @Summary      列出可备份项
// @Description  返回工作目录下所有可选的待备份项列表
// @Tags         Admin.Backup
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]string}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/backups/work-dir/options [get]
// @Description  此方法由 HandleWorkDir 内部分发器调用
func (b *BackupHandler) ListToBackupItems(ctx *gin.Context) (interface{}, error) {
	return b.BackupService.ListToBackupItems(ctx)
}

func (b *BackupHandler) HandleWorkDir(ctx *gin.Context) {
	path := ctx.Request.URL.Path
	if path == "/api/admin/backups/work-dir/fetch" {
		wrapHandler(b.GetWorkDirBackup)(ctx)
		return
	}
	if path == "/api/admin/backups/work-dir/options" || path == "/api/admin/backups/work-dir/options/" {
		wrapHandler(b.ListToBackupItems)(ctx)
		return
	}
	b.DownloadBackups(ctx)
}

func (b *BackupHandler) DownloadBackups(ctx *gin.Context) {
	filename := ctx.Param("path")
	if filename == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, &dto.BaseDTO{
			Status:  http.StatusBadRequest,
			Message: "Filename parameter does not exist",
		})
		return
	}
	filePath, err := b.BackupService.GetBackupFilePath(ctx, config.BackupDir, filename)
	if err != nil {
		log.CtxErrorf(ctx, "err=%+v", err)
		status := xerr.GetHTTPStatus(err)
		ctx.JSON(status, &dto.BaseDTO{Status: status, Message: xerr.GetMessage(err)})
	}
	ctx.File(filePath)
}

// DeleteBackups godoc
// @Summary      删除工作目录备份
// @Description  根据 filename 删除指定的工作目录备份文件
// @Tags         Admin.Backup
// @Produce      json
// @Param        filename  query     string  true  "备份文件名"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/backups/work-dir [delete]
func (b *BackupHandler) DeleteBackups(ctx *gin.Context) (interface{}, error) {
	filename, err := util.MustGetQueryString(ctx, "filename")
	if err != nil {
		return nil, err
	}
	return nil, b.BackupService.DeleteFile(ctx, config.BackupDir, filename)
}

// ImportMarkdown godoc
// @Summary      导入 Markdown 备份
// @Description  上传 Markdown 备份文件并导入到系统
// @Tags         Admin.Backup
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "上传的 Markdown 备份文件"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/backups/markdown/import [post]
func (b *BackupHandler) ImportMarkdown(ctx *gin.Context) (interface{}, error) {
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		return nil, xerr.WithMsg(err, "上传文件错误").WithStatus(xerr.StatusBadRequest)
	}
	filenameExt := path.Ext(fileHeader.Filename)
	if filenameExt != ".md" && filenameExt != ".markdown" && filenameExt != ".mdown" {
		return nil, xerr.WithMsg(err, "Unsupported format").WithStatus(xerr.StatusBadRequest)
	}
	return nil, b.BackupService.ImportMarkdown(ctx, fileHeader)
}

// ExportData godoc
// @Summary      导出数据备份
// @Description  将系统数据导出为 JSON 备份文件
// @Tags         Admin.Backup
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.BackupDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/backups/data [post]
func (b *BackupHandler) ExportData(ctx *gin.Context) (interface{}, error) {
	return b.BackupService.ExportData(ctx)
}

func (b *BackupHandler) HandleData(ctx *gin.Context) {
	path := ctx.Request.URL.Path
	if path == "/api/admin/backups/data/fetch" {
		wrapHandler(b.GetDataBackup)(ctx)
		return
	}
	if path == "/api/admin/backups/data" || path == "/api/admin/backups/data/" {
		wrapHandler(b.ListExportData)(ctx)
		return
	}
	b.DownloadData(ctx)
}

// ListExportData godoc
// @Summary      列出数据备份
// @Description  返回所有已导出的数据备份文件列表
// @Tags         Admin.Backup
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.BackupDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/backups/data [get]
// @Description  此方法由 HandleData 内部分发器调用
func (b *BackupHandler) ListExportData(ctx *gin.Context) (interface{}, error) {
	return b.BackupService.ListFiles(ctx, config.DataExportDir, service.JSONData)
}

func (b *BackupHandler) DownloadData(ctx *gin.Context) {
	filename := ctx.Param("path")
	if filename == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, &dto.BaseDTO{
			Status:  http.StatusBadRequest,
			Message: "Filename parameter does not exist",
		})
	}
	filePath, err := b.BackupService.GetBackupFilePath(ctx, config.DataExportDir, filename)
	if err != nil {
		log.CtxErrorf(ctx, "err=%+v", err)
		status := xerr.GetHTTPStatus(err)
		ctx.JSON(status, &dto.BaseDTO{Status: status, Message: xerr.GetMessage(err)})
	}
	ctx.File(filePath)
}

// DeleteDataFile godoc
// @Summary      删除数据备份文件
// @Description  根据 filename 删除指定的数据备份文件
// @Tags         Admin.Backup
// @Produce      json
// @Param        filename  query     string  true  "备份文件名"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/backups/data [delete]
func (b *BackupHandler) DeleteDataFile(ctx *gin.Context) (interface{}, error) {
	filename, ok := ctx.GetQuery("filename")
	if !ok || filename == "" {
		return nil, xerr.BadParam.New("no filename param").WithStatus(xerr.StatusBadRequest).WithMsg("no filename param")
	}
	return nil, b.BackupService.DeleteFile(ctx, config.DataExportDir, filename)
}

// ExportMarkdown godoc
// @Summary      导出 Markdown 备份
// @Description  将文章/页面等内容导出为 Markdown 备份
// @Tags         Admin.Backup
// @Accept       json
// @Produce      json
// @Param        exportMarkdown  body     param.ExportMarkdown  true  "导出参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.BackupDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/backups/markdown/export [post]
func (b *BackupHandler) ExportMarkdown(ctx *gin.Context) (interface{}, error) {
	var exportMarkdownParam param.ExportMarkdown
	err := ctx.ShouldBindJSON(&exportMarkdownParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest)
	}
	return b.BackupService.ExportMarkdown(ctx, exportMarkdownParam.NeedFrontMatter)
}

// ListMarkdowns godoc
// @Summary      列出 Markdown 备份
// @Description  返回所有已导出的 Markdown 备份文件列表
// @Tags         Admin.Backup
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.BackupDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/backups/markdown/export [get]
func (b *BackupHandler) ListMarkdowns(ctx *gin.Context) (interface{}, error) {
	return b.BackupService.ListFiles(ctx, config.BackupMarkdownDir, service.Markdown)
}

// DeleteMarkdowns godoc
// @Summary      删除 Markdown 备份
// @Description  根据 filename 删除指定的 Markdown 备份文件
// @Tags         Admin.Backup
// @Produce      json
// @Param        filename  query     string  true  "备份文件名"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/backups/markdown/export [delete]
func (b *BackupHandler) DeleteMarkdowns(ctx *gin.Context) (interface{}, error) {
	filename, err := util.MustGetQueryString(ctx, "filename")
	if err != nil {
		return nil, err
	}
	return nil, b.BackupService.DeleteFile(ctx, config.BackupMarkdownDir, filename)
}

// DownloadMarkdown godoc
// @Summary      下载 Markdown 备份
// @Description  根据 filename 下载指定的 Markdown 备份文件
// @Tags         Admin.Backup
// @Produce      octet-stream
// @Param        filename  path     string  true  "备份文件名"
// @Security     AdminApiKey
// @Success      200  {string}  string
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/backups/markdown/export/{filename} [get]
func (b *BackupHandler) DownloadMarkdown(ctx *gin.Context) {
	filename := ctx.Param("filename")
	if filename == "" {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, &dto.BaseDTO{
			Status:  http.StatusBadRequest,
			Message: "Filename parameter does not exist",
		})
		return
	}
	filePath, err := b.BackupService.GetBackupFilePath(ctx, config.BackupMarkdownDir, filename)
	if err != nil {
		log.CtxErrorf(ctx, "err=%+v", err)
		status := xerr.GetHTTPStatus(err)
		ctx.JSON(status, &dto.BaseDTO{Status: status, Message: xerr.GetMessage(err)})
	}
	ctx.File(filePath)
}

type wrapperHandler func(ctx *gin.Context) (interface{}, error)

func wrapHandler(handler wrapperHandler) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		data, err := handler(ctx)
		if err != nil {
			log.CtxErrorf(ctx, "err=%+v", err)
			status := xerr.GetHTTPStatus(err)
			ctx.JSON(status, &dto.BaseDTO{Status: status, Message: xerr.GetMessage(err)})
			return
		}

		ctx.JSON(http.StatusOK, &dto.BaseDTO{
			Status:  http.StatusOK,
			Data:    data,
			Message: "OK",
		})
	}
}
