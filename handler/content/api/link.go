package api

import (
	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
)

type LinkHandler struct {
	LinkService service.LinkService
}

func NewLinkHandler(linkService service.LinkService) *LinkHandler {
	return &LinkHandler{
		LinkService: linkService,
	}
}

type linkParam struct {
	*param.Sort
}

// ListLinks godoc
// @Summary      查询友情链接列表
// @Description  返回全部友情链接,默认按创建时间倒序
// @Tags         Content.Link
// @Produce      json
// @Param        sort  query     []string  false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Link}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/links [get]
func (l *LinkHandler) ListLinks(ctx *gin.Context) (interface{}, error) {
	p := linkParam{}
	if err := ctx.ShouldBindQuery(&p); err != nil {
		return nil, err
	}

	if p.Sort == nil || len(p.Fields) == 0 {
		p.Sort = &param.Sort{
			Fields: []string{"createTime,desc"},
		}
	}
	links, err := l.LinkService.List(ctx, p.Sort)
	if err != nil {
		return nil, err
	}
	return l.LinkService.ConvertToDTOs(ctx, links), nil
}

// LinkTeamVO godoc
// @Summary      按分组返回友情链接
// @Description  返回按 team 分组的友情链接列表
// @Tags         Content.Link
// @Produce      json
// @Param        sort  query     []string  false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Success      200  {object}  dto.BaseDTO{data=[]vo.LinkTeamVO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/links/team_view [get]
func (l *LinkHandler) LinkTeamVO(ctx *gin.Context) (interface{}, error) {
	p := linkParam{}
	if err := ctx.ShouldBindQuery(&p); err != nil {
		return nil, err
	}

	if p.Sort == nil || len(p.Fields) == 0 {
		p.Sort = &param.Sort{
			Fields: []string{"createTime,desc"},
		}
	}
	links, err := l.LinkService.List(ctx, p.Sort)
	if err != nil {
		return nil, err
	}
	return l.LinkService.ConvertToLinkTeamVO(ctx, links), nil
}
