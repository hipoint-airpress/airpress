package admin

import (
	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/handler/binding"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type AttachmentHandler struct {
	AttachmentService service.AttachmentService
}

func NewAttachmentHandler(attachmentService service.AttachmentService) *AttachmentHandler {
	return &AttachmentHandler{
		AttachmentService: attachmentService,
	}
}

// QueryAttachment godoc
// @Summary      分页查询附件列表
// @Description  支持按关键词、媒体类型、附件类型过滤,支持排序与分页
// @Tags         Admin.Attachment
// @Accept       json
// @Produce      json
// @Param        page        query     int      false  "页码(从0开始)"  example(0)
// @Param        size        query     int      false  "每页数量"      example(10)
// @Param        sort        query     []string false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Param        keyword     query     string   false  "名称关键词"
// @Param        mediaType   query     string   false  "媒体类型"
// @Param        attachmentType  query  int    false  "附件类型"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]dto.AttachmentDTO}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/attachments [get]
func (a *AttachmentHandler) QueryAttachment(ctx *gin.Context) (interface{}, error) {
	queryParam := &param.AttachmentQuery{}
	err := ctx.ShouldBindWith(queryParam, binding.CustomFormBinding)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("param error ")
	}
	attachments, totalCount, err := a.AttachmentService.Page(ctx, queryParam)
	if err != nil {
		return nil, err
	}
	attachmentDTOs, err := a.AttachmentService.ConvertToDTOs(ctx, attachments)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(attachmentDTOs, totalCount, queryParam.Page), nil
}

// GetAttachmentByID godoc
// @Summary      根据ID获取附件
// @Description  返回指定 ID 的附件详情
// @Tags         Admin.Attachment
// @Produce      json
// @Param        id  path     int  true  "附件ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.AttachmentDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/attachments/{id} [get]
func (a *AttachmentHandler) GetAttachmentByID(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	if id < 0 {
		return nil, xerr.BadParam.New("id < 0").WithStatus(xerr.StatusBadRequest).WithMsg("param error")
	}
	return a.AttachmentService.GetAttachment(ctx, id)
}

// UploadAttachment godoc
// @Summary      上传单个附件
// @Description  通过 multipart 表单上传单个文件,返回附件详情
// @Tags         Admin.Attachment
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "上传的文件"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.AttachmentDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/attachments/upload [post]
func (a *AttachmentHandler) UploadAttachment(ctx *gin.Context) (interface{}, error) {
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		return nil, xerr.WithMsg(err, "上传文件错误").WithStatus(xerr.StatusBadRequest)
	}
	return a.AttachmentService.Upload(ctx, fileHeader)
}

// UploadAttachments godoc
// @Summary      批量上传附件
// @Description  通过 multipart 表单上传多个文件,返回附件详情列表
// @Tags         Admin.Attachment
// @Accept       multipart/form-data
// @Produce      json
// @Param        files  formData  []file  true  "上传的文件列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.AttachmentDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/attachments/uploads [post]
func (a *AttachmentHandler) UploadAttachments(ctx *gin.Context) (interface{}, error) {
	form, _ := ctx.MultipartForm()
	if len(form.File) == 0 {
		return nil, xerr.BadParam.New("empty files").WithStatus(xerr.StatusBadRequest).WithMsg("empty files")
	}
	files := form.File["files"]
	attachmentDTOs := make([]*dto.AttachmentDTO, 0)
	for _, file := range files {
		attachment, err := a.AttachmentService.Upload(ctx, file)
		if err != nil {
			return nil, err
		}
		attachmentDTOs = append(attachmentDTOs, attachment)
	}
	return attachmentDTOs, nil
}

// UpdateAttachment godoc
// @Summary      更新附件
// @Description  根据附件ID更新附件信息
// @Tags         Admin.Attachment
// @Accept       json
// @Produce      json
// @Param        id              path     int                    true  "附件ID"  example(1)
// @Param        attachmentUpdate  body   param.AttachmentUpdate  true  "附件更新参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.AttachmentDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/attachments/{id} [put]
func (a *AttachmentHandler) UpdateAttachment(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}

	updateParam := &param.AttachmentUpdate{}
	err = ctx.ShouldBind(updateParam)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("param error ")
	}
	return a.AttachmentService.Update(ctx, id, updateParam)
}

// DeleteAttachment godoc
// @Summary      删除附件
// @Description  根据附件ID删除指定附件
// @Tags         Admin.Attachment
// @Produce      json
// @Param        id  path     int  true  "附件ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/attachments/{id} [delete]
func (a *AttachmentHandler) DeleteAttachment(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	return a.AttachmentService.Delete(ctx, id)
}

// DeleteAttachmentInBatch godoc
// @Summary      批量删除附件
// @Description  根据附件ID列表批量删除附件
// @Tags         Admin.Attachment
// @Accept       json
// @Produce      json
// @Param        ids  body     []int  true  "附件ID列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/attachments [delete]
func (a *AttachmentHandler) DeleteAttachmentInBatch(ctx *gin.Context) (interface{}, error) {
	ids := make([]int32, 0)
	err := ctx.ShouldBind(&ids)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	return a.AttachmentService.DeleteBatch(ctx, ids)
}

// GetAllMediaType godoc
// @Summary      获取所有媒体类型
// @Description  返回附件库中所有出现过的媒体类型列表
// @Tags         Admin.Attachment
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]string}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/attachments/media_types [get]
func (a *AttachmentHandler) GetAllMediaType(ctx *gin.Context) (interface{}, error) {
	return a.AttachmentService.GetAllMediaTypes(ctx)
}

// GetAllTypes godoc
// @Summary      获取所有附件类型
// @Description  返回系统支持的所有附件类型列表
// @Tags         Admin.Attachment
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]int}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/attachments/types [get]
func (a *AttachmentHandler) GetAllTypes(ctx *gin.Context) (interface{}, error) {
	attachmentTypes, err := a.AttachmentService.GetAllTypes(ctx)
	if err != nil {
		return nil, err
	}
	return attachmentTypes, nil
}
