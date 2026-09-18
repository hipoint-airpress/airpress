package admin

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/hipoint-airpress/airpress/handler/binding"
	"github.com/hipoint-airpress/airpress/handler/trans"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type LinkHandler struct {
	LinkService service.LinkService
}

func NewLinkHandler(linkService service.LinkService) *LinkHandler {
	return &LinkHandler{
		LinkService: linkService,
	}
}

// ListLinks godoc
// @Summary      查询所有友情链接
// @Description  返回所有友情链接列表,支持按 team 与 priority 排序
// @Tags         Admin.Link
// @Accept       json
// @Produce      json
// @Param        sort  query     []string  false  "排序字段,如 team,desc"  collectionFormat(multi)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Link}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/links [get]
func (l *LinkHandler) ListLinks(ctx *gin.Context) (interface{}, error) {
	sort := param.Sort{}
	err := ctx.ShouldBindWith(&sort, binding.CustomFormBinding)
	if err != nil {
		return nil, xerr.WithMsg(err, "sort parameter error").WithStatus(xerr.StatusBadRequest)
	}
	if len(sort.Fields) == 0 {
		sort.Fields = append(sort.Fields, "team,desc", "priority,asc")
	} else {
		sort.Fields = append(sort.Fields, "priority,asc")
	}
	links, err := l.LinkService.List(ctx, &sort)
	if err != nil {
		return nil, err
	}
	return l.LinkService.ConvertToDTOs(ctx, links), nil
}

// GetLinkByID godoc
// @Summary      根据ID获取友情链接
// @Description  返回指定 ID 的友情链接详情
// @Tags         Admin.Link
// @Produce      json
// @Param        id  path     int  true  "友情链接ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Link}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/links/{id} [get]
func (l *LinkHandler) GetLinkByID(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	link, err := l.LinkService.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return l.LinkService.ConvertToDTO(ctx, link), nil
}

// CreateLink godoc
// @Summary      创建友情链接
// @Description  创建一个新友情链接,返回创建后的详情
// @Tags         Admin.Link
// @Accept       json
// @Produce      json
// @Param        link  body     param.Link  true  "友情链接参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Link}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/links [post]
func (l *LinkHandler) CreateLink(ctx *gin.Context) (interface{}, error) {
	linkParam := &param.Link{}
	err := ctx.ShouldBindJSON(linkParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	link, err := l.LinkService.Create(ctx, linkParam)
	if err != nil {
		return nil, err
	}
	return l.LinkService.ConvertToDTO(ctx, link), nil
}

// UpdateLink godoc
// @Summary      更新友情链接
// @Description  根据友情链接ID更新友情链接信息
// @Tags         Admin.Link
// @Accept       json
// @Produce      json
// @Param        id    path     int          true  "友情链接ID"  example(1)
// @Param        link  body     param.Link   true  "友情链接参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Link}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/links/{id} [put]
func (l *LinkHandler) UpdateLink(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	linkParam := &param.Link{}
	err = ctx.ShouldBindJSON(linkParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	link, err := l.LinkService.Update(ctx, id, linkParam)
	if err != nil {
		return nil, err
	}
	return l.LinkService.ConvertToDTO(ctx, link), nil
}

// DeleteLink godoc
// @Summary      删除友情链接
// @Description  根据友情链接ID删除指定友情链接
// @Tags         Admin.Link
// @Produce      json
// @Param        id  path     int  true  "友情链接ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/links/{id} [delete]
func (l *LinkHandler) DeleteLink(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	return nil, l.LinkService.Delete(ctx, id)
}

// ListLinkTeams godoc
// @Summary      查询友情链接分组
// @Description  返回所有友情链接的分组(team)列表
// @Tags         Admin.Link
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]string}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/links/teams [get]
func (l *LinkHandler) ListLinkTeams(ctx *gin.Context) (interface{}, error) {
	return l.LinkService.ListTeams(ctx)
}
