package admin

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/hipoint-airpress/airpress/handler/binding"
	"github.com/hipoint-airpress/airpress/handler/trans"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type JournalHandler struct {
	JournalService service.JournalService
}

func NewJournalHandler(journalService service.JournalService) *JournalHandler {
	return &JournalHandler{
		JournalService: journalService,
	}
}

// ListJournal godoc
// @Summary      分页查询日志列表
// @Description  支持按关键词过滤,支持排序与分页
// @Tags         Admin.Journal
// @Accept       json
// @Produce      json
// @Param        page     query     int        false  "页码(从0开始)"  example(0)
// @Param        size     query     int        false  "每页数量"        example(10)
// @Param        sort     query     []string   false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Param        keyword  query     string     false  "日志关键词"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]dto.JournalWithComment}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/journals [get]
func (j *JournalHandler) ListJournal(ctx *gin.Context) (interface{}, error) {
	var journalQuery param.JournalQuery
	err := ctx.ShouldBindWith(&journalQuery, binding.CustomFormBinding)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	journalQuery.Sort = &param.Sort{
		Fields: []string{"createTime,desc"},
	}
	journals, totalCount, err := j.JournalService.ListJournal(ctx, journalQuery)
	if err != nil {
		return nil, err
	}
	journalDTOs, err := j.JournalService.ConvertToWithCommentDTOList(ctx, journals)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(journalDTOs, totalCount, journalQuery.Page), nil
}

// ListLatestJournal godoc
// @Summary      查询最新日志
// @Description  返回指定数量的最新日志列表
// @Tags         Admin.Journal
// @Produce      json
// @Param        top  query     int  false  "返回数量"  example(10)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.JournalWithComment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/journals/latest [get]
func (j *JournalHandler) ListLatestJournal(ctx *gin.Context) (interface{}, error) {
	top, err := util.MustGetQueryInt(ctx, "top")
	if err != nil {
		top = 10
	}
	journalQuery := param.JournalQuery{
		Sort: &param.Sort{Fields: []string{"createTime,desc"}},
		Page: param.Page{PageNum: 0, PageSize: top},
	}
	journals, _, err := j.JournalService.ListJournal(ctx, journalQuery)
	if err != nil {
		return nil, err
	}
	return j.JournalService.ConvertToWithCommentDTOList(ctx, journals)
}

// CreateJournal godoc
// @Summary      创建日志
// @Description  创建一条新日志,返回创建后的详情
// @Tags         Admin.Journal
// @Accept       json
// @Produce      json
// @Param        journal  body     param.Journal  true  "日志参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Journal}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/journals [post]
func (j *JournalHandler) CreateJournal(ctx *gin.Context) (interface{}, error) {
	var journalParam param.Journal
	err := ctx.ShouldBindJSON(&journalParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	if journalParam.Content == "" {
		journalParam.Content = journalParam.SourceContent
	}
	journal, err := j.JournalService.Create(ctx, &journalParam)
	if err != nil {
		return nil, err
	}
	return j.JournalService.ConvertToDTO(journal), nil
}

// UpdateJournal godoc
// @Summary      更新日志
// @Description  根据日志ID更新日志内容
// @Tags         Admin.Journal
// @Accept       json
// @Produce      json
// @Param        journalID  path     int            true  "日志ID"  example(1)
// @Param        journal    body     param.Journal  true  "日志参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Journal}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/journals/{journalID} [put]
func (j *JournalHandler) UpdateJournal(ctx *gin.Context) (interface{}, error) {
	var journalParam param.Journal
	err := ctx.ShouldBindJSON(&journalParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}

	journalID, err := util.ParamInt32(ctx, "journalID")
	if err != nil {
		return nil, err
	}
	return j.JournalService.Update(ctx, journalID, &journalParam)
}

// DeleteJournal godoc
// @Summary      删除日志
// @Description  根据日志ID删除指定日志
// @Tags         Admin.Journal
// @Produce      json
// @Param        journalID  path     int  true  "日志ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/journals/{journalID} [delete]
func (j *JournalHandler) DeleteJournal(ctx *gin.Context) (interface{}, error) {
	journalID, err := util.ParamInt32(ctx, "journalID")
	if err != nil {
		return nil, err
	}
	return nil, j.JournalService.Delete(ctx, journalID)
}
