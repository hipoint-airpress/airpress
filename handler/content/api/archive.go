package api

import (
	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/service/assembler"
)

type ArchiveHandler struct {
	PostService   service.PostService
	PostAssembler assembler.PostAssembler
}

func NewArchiveHandler(postService service.PostService, postAssemeber assembler.PostAssembler) *ArchiveHandler {
	return &ArchiveHandler{
		PostService:   postService,
		PostAssembler: postAssemeber,
	}
}

// ListYearArchives godoc
// @Summary      按年份归档文章
// @Description  返回已发布文章按年份分组的归档列表
// @Tags         Content.Archive
// @Produce      json
// @Success      200  {object}  dto.BaseDTO{data=[]vo.ArchiveYear}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/archives/years [get]
func (a *ArchiveHandler) ListYearArchives(ctx *gin.Context) (interface{}, error) {
	posts, err := a.PostService.GetByStatus(ctx, []consts.PostStatus{consts.PostStatusPublished}, consts.PostTypePost, nil)
	if err != nil {
		return nil, err
	}
	return a.PostAssembler.ConvertToArchiveYearVOs(ctx, posts)
}

// ListMonthArchives godoc
// @Summary      按月份归档文章
// @Description  返回已发布文章按月份分组的归档列表
// @Tags         Content.Archive
// @Produce      json
// @Success      200  {object}  dto.BaseDTO{data=[]vo.ArchiveMonth}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/archives/months [get]
func (a *ArchiveHandler) ListMonthArchives(ctx *gin.Context) (interface{}, error) {
	posts, err := a.PostService.GetByStatus(ctx, []consts.PostStatus{consts.PostStatusPublished}, consts.PostTypePost, nil)
	if err != nil {
		return nil, err
	}
	return a.PostAssembler.ConvertTOArchiveMonthVOs(ctx, posts)
}
