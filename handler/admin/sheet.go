package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/handler/trans"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/service/assembler"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type SheetHandler struct {
	SheetService   service.SheetService
	PostService    service.PostService
	SheetAssembler assembler.SheetAssembler
}

func NewSheetHandler(sheetService service.SheetService, postService service.PostService, sheetAssembler assembler.SheetAssembler) *SheetHandler {
	return &SheetHandler{
		SheetService:   sheetService,
		PostService:    postService,
		SheetAssembler: sheetAssembler,
	}
}

// GetSheetByID godoc
// @Summary      根据页面ID获取详情
// @Description  返回页面详情,包含正文、评论数等
// @Tags         Admin.Sheet
// @Produce      json
// @Param        sheetID  path     int  true  "页面ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=vo.SheetDetail}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets/{sheetID} [get]
func (s *SheetHandler) GetSheetByID(ctx *gin.Context) (interface{}, error) {
	sheetID, err := util.ParamInt32(ctx, "sheetID")
	if err != nil {
		return nil, err
	}
	sheet, err := s.SheetService.GetByPostID(ctx, sheetID)
	if err != nil {
		return nil, err
	}
	return s.SheetAssembler.ConvertToDetailVO(ctx, sheet)
}

// ListSheet godoc
// @Summary      分页查询页面列表
// @Description  支持分页查询所有页面
// @Tags         Admin.Sheet
// @Accept       json
// @Produce      json
// @Param        page  query     int        false  "页码(从0开始)"  example(0)
// @Param        size  query     int        false  "每页数量"        example(10)
// @Param        sort  query     string     false  "排序字段"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.SheetList}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets [get]
func (s *SheetHandler) ListSheet(ctx *gin.Context) (interface{}, error) {
	type SheetParam struct {
		param.Page
		Sort string `json:"sort"`
	}
	var sheetParam SheetParam
	err := ctx.ShouldBind(&sheetParam)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	sheets, totalCount, err := s.SheetService.Page(ctx, sheetParam.Page, &param.Sort{Fields: []string{"createTime,desc"}})
	if err != nil {
		return nil, err
	}
	sheetVOs, err := s.SheetAssembler.ConvertToListVO(ctx, sheets)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(sheetVOs, totalCount, sheetParam.Page), nil
}

// IndependentSheets godoc
// @Summary      查询独立页面列表
// @Description  返回所有独立页面(sheets)列表
// @Tags         Admin.Sheet
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.IndependentSheet}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets/independent [get]
func (s *SheetHandler) IndependentSheets(ctx *gin.Context) (interface{}, error) {
	return s.SheetService.ListIndependentSheets(ctx)
}

// CreateSheet godoc
// @Summary      创建页面
// @Description  创建一个新页面,返回创建后的详情
// @Tags         Admin.Sheet
// @Accept       json
// @Produce      json
// @Param        sheet  body     param.Sheet  true  "页面参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=vo.SheetDetail}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets [post]
func (s *SheetHandler) CreateSheet(ctx *gin.Context) (interface{}, error) {
	var sheetParam param.Sheet
	err := ctx.ShouldBindJSON(&sheetParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest)
	}
	sheet, err := s.SheetService.Create(ctx, &sheetParam)
	if err != nil {
		return nil, err
	}
	sheetDetailVO, err := s.SheetAssembler.ConvertToDetailVO(ctx, sheet)
	if err != nil {
		return nil, err
	}
	return sheetDetailVO, nil
}

// UpdateSheet godoc
// @Summary      更新页面
// @Description  根据页面ID更新页面内容
// @Tags         Admin.Sheet
// @Accept       json
// @Produce      json
// @Param        sheetID  path     int           true  "页面ID"  example(1)
// @Param        sheet    body     param.Sheet   true  "页面参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=vo.SheetDetail}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets/{sheetID} [put]
func (s *SheetHandler) UpdateSheet(ctx *gin.Context) (interface{}, error) {
	var sheetParam param.Sheet
	err := ctx.ShouldBindJSON(&sheetParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}

	sheetID, err := util.ParamInt32(ctx, "sheetID")
	if err != nil {
		return nil, err
	}
	postDetailVO, err := s.SheetService.Update(ctx, sheetID, &sheetParam)
	if err != nil {
		return nil, err
	}
	return postDetailVO, nil
}

// UpdateSheetStatus godoc
// @Summary      更新页面状态
// @Description  根据页面ID和状态更新页面状态
// @Tags         Admin.Sheet
// @Produce      json
// @Param        sheetID  path     int     true  "页面ID"  example(1)
// @Param        status    path     string  true  "状态值"  example(published)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=vo.SheetDetail}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets/{sheetID}/{status} [put]
func (s *SheetHandler) UpdateSheetStatus(ctx *gin.Context) (interface{}, error) {
	sheetID, err := util.ParamInt32(ctx, "sheetID")
	if err != nil {
		return nil, err
	}
	statusStr, err := util.ParamString(ctx, "status")
	if err != nil {
		return nil, err
	}
	status, err := consts.PostStatusFromString(statusStr)
	if err != nil {
		return nil, err
	}
	if status < consts.PostStatusPublished || status > consts.PostStatusIntimate {
		return nil, xerr.WithStatus(nil, xerr.StatusBadRequest).WithMsg("status error")
	}
	return s.SheetService.UpdateStatus(ctx, sheetID, status)
}

// UpdateSheetDraft godoc
// @Summary      更新页面草稿内容
// @Description  根据页面ID更新页面的草稿内容(正文/原始内容)
// @Tags         Admin.Sheet
// @Accept       json
// @Produce      json
// @Param        sheetID       path     int               true  "页面ID"  example(1)
// @Param        postContent   body     param.PostContent  true  "草稿内容参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.PostDetail}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets/{sheetID}/status/draft/content [put]
func (s *SheetHandler) UpdateSheetDraft(ctx *gin.Context) (interface{}, error) {
	sheetID, err := util.ParamInt32(ctx, "sheetID")
	if err != nil {
		return nil, err
	}
	var postContentParam param.PostContent
	err = ctx.ShouldBindJSON(&postContentParam)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("content param error")
	}
	post, err := s.SheetService.UpdateDraftContent(ctx, sheetID, postContentParam.Content, postContentParam.OriginalContent)
	if err != nil {
		return nil, err
	}
	return s.SheetAssembler.ConvertToDetailDTO(ctx, post)
}

// DeleteSheet godoc
// @Summary      删除页面
// @Description  根据页面ID删除指定页面
// @Tags         Admin.Sheet
// @Produce      json
// @Param        sheetID  path     int  true  "页面ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets/{sheetID} [delete]
func (s *SheetHandler) DeleteSheet(ctx *gin.Context) (interface{}, error) {
	sheetID, err := util.ParamInt32(ctx, "sheetID")
	if err != nil {
		return nil, err
	}
	return nil, s.SheetService.Delete(ctx, sheetID)
}

// PreviewSheet godoc
// @Summary      预览页面
// @Description  根据页面ID生成预览并返回预览路径
// @Tags         Admin.Sheet
// @Produce      plain
// @Param        sheetID  path     int  true  "页面ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {string}  string
// @Failure      400  {string}  string
// @Failure      500  {string}  string
// @Router       /admin/sheets/preview/{sheetID} [get]
func (s *SheetHandler) PreviewSheet(ctx *gin.Context) {
	sheetID, err := util.ParamInt32(ctx, "sheetID")
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		_ = ctx.Error(err)
		return
	}

	previewPath, err := s.SheetService.Preview(ctx, sheetID)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		_ = ctx.Error(err)
		return
	}
	ctx.String(http.StatusOK, previewPath)
}
