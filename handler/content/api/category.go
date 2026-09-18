package api

import (
	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/handler/binding"
	"github.com/hipoint-airpress/airpress/handler/content/authentication"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/service/assembler"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type CategoryHandler struct {
	PostService            service.PostService
	CategoryService        service.CategoryService
	CategoryAuthentication authentication.CategoryAuthentication
	PostAssembler          assembler.PostAssembler
}

func NewCategoryHandler(postService service.PostService, categoryService service.CategoryService, categoryAuthentication *authentication.CategoryAuthentication, postAssembler assembler.PostAssembler) *CategoryHandler {
	return &CategoryHandler{
		PostService:            postService,
		CategoryService:        categoryService,
		CategoryAuthentication: *categoryAuthentication,
		PostAssembler:          postAssembler,
	}
}

// ListCategories godoc
// @Summary      查询分类列表
// @Description  返回所有分类,more=true 时返回带文章数的分类
// @Tags         Content.Category
// @Produce      json
// @Param        sort  query     []string  false  "排序字段,如 updateTime,desc"  collectionFormat(multi)
// @Param        more  query     bool      false  "true返回带文章数分类,false返回精简分类"
// @Success      200  {object}  dto.BaseDTO{data=[]dto.CategoryDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/categories [get]
func (c *CategoryHandler) ListCategories(ctx *gin.Context) (interface{}, error) {
	categoryQuery := struct {
		*param.Sort
		More *bool `json:"more" form:"more"`
	}{}

	err := ctx.ShouldBindQuery(&categoryQuery)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	if categoryQuery.Sort == nil || len(categoryQuery.Fields) == 0 {
		categoryQuery.Sort = &param.Sort{Fields: []string{"updateTime,desc"}}
	}
	if categoryQuery.More != nil && *categoryQuery.More {
		return c.CategoryService.ListCategoryWithPostCountDTO(ctx, categoryQuery.Sort)
	}
	categories, err := c.CategoryService.ListAll(ctx, categoryQuery.Sort)
	if err != nil {
		return nil, err
	}
	return c.CategoryService.ConvertToCategoryDTOs(ctx, categories)
}

// ListPosts godoc
// @Summary      根据分类 slug 查询文章列表
// @Description  分页返回指定分类下的已发布文章,如分类为私密需密码鉴权
// @Tags         Content.Category
// @Produce      json
// @Param        slug      path     string    true   "分类 slug"
// @Param        page      query     int      false  "页码(从0开始)"          example(0)
// @Param        size      query     int      false  "每页数量"              example(10)
// @Param        sort      query     []string false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Param        keyword   query     string   false  "标题关键词"
// @Param        password  query     string   false  "私密分类访问密码"
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.Post}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/categories/{slug}/posts [get]
func (c *CategoryHandler) ListPosts(ctx *gin.Context) (interface{}, error) {
	slug, err := util.ParamString(ctx, "slug")
	if err != nil {
		return nil, err
	}
	category, err := c.CategoryService.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	postQuery := param.PostQuery{}
	err = ctx.ShouldBindWith(&postQuery, binding.CustomFormBinding)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	if postQuery.Sort == nil {
		postQuery.Sort = &param.Sort{Fields: []string{"topPriority,desc", "updateTime,desc"}}
	}
	password, _ := util.MustGetQueryString(ctx, "password")

	if category.Type == consts.CategoryTypeIntimate {
		token, _ := ctx.Cookie("authentication")
		if authenticated, _ := c.CategoryAuthentication.IsAuthenticated(ctx, token, category.ID); !authenticated {
			token, err := c.CategoryAuthentication.Authenticate(ctx, token, category.ID, password)
			if err != nil {
				return nil, err
			}
			ctx.SetCookie("authentication", token, 1800, "/", "", false, true)
		}
	}
	postQuery.WithPassword = util.BoolPtr(false)
	postQuery.Statuses = []*consts.PostStatus{consts.PostStatusPublished.Ptr(), consts.PostStatusIntimate.Ptr()}
	posts, totalCount, err := c.PostService.Page(ctx, postQuery)
	if err != nil {
		return nil, err
	}
	postVOs, err := c.PostAssembler.ConvertToListVO(ctx, posts)
	return dto.NewPage(postVOs, totalCount, postQuery.Page), err
}
