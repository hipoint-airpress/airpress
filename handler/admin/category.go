package admin

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/hipoint-airpress/airpress/handler/trans"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type CategoryHandler struct {
	CategoryService service.CategoryService
}

func NewCategoryHandler(categoryService service.CategoryService) *CategoryHandler {
	return &CategoryHandler{
		CategoryService: categoryService,
	}
}

// GetCategoryByID godoc
// @Summary      根据ID获取分类
// @Description  返回指定 ID 的分类详情
// @Tags         Admin.Category
// @Produce      json
// @Param        categoryID  path     int  true  "分类ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.CategoryDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/categories/{categoryID} [get]
func (c *CategoryHandler) GetCategoryByID(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "categoryID")
	if err != nil {
		return nil, err
	}
	category, err := c.CategoryService.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.CategoryService.ConvertToCategoryDTO(ctx, category)
}

// ListAllCategory godoc
// @Summary      查询所有分类
// @Description  支持按 more 参数返回带文章数的详情列表或精简列表,支持排序
// @Tags         Admin.Category
// @Accept       json
// @Produce      json
// @Param        sort  query     []string  false  "排序字段,如 priority,asc"  collectionFormat(multi)
// @Param        more  query     bool      false  "true返回带文章数的详情"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.CategoryDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/categories [get]
func (c *CategoryHandler) ListAllCategory(ctx *gin.Context) (interface{}, error) {
	categoryQuery := struct {
		*param.Sort
		More *bool `json:"more" form:"more"`
	}{}

	err := ctx.ShouldBindQuery(&categoryQuery)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	if categoryQuery.Sort == nil || len(categoryQuery.Fields) == 0 {
		categoryQuery.Sort = &param.Sort{Fields: []string{"priority,asc"}}
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

// ListAsTree godoc
// @Summary      树形分类列表
// @Description  返回所有分类的树形结构
// @Tags         Admin.Category
// @Accept       json
// @Produce      json
// @Param        sort  query     []string  false  "排序字段,如 priority,asc"  collectionFormat(multi)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]vo.CategoryVO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/categories/tree_view [get]
func (c *CategoryHandler) ListAsTree(ctx *gin.Context) (interface{}, error) {
	var sort param.Sort
	err := ctx.ShouldBindQuery(&sort)
	if err != nil {
		return nil, err
	}
	if len(sort.Fields) == 0 {
		sort.Fields = append(sort.Fields, "priority,asc")
	}
	return c.CategoryService.ListAsTree(ctx, &sort, false)
}

// CreateCategory godoc
// @Summary      创建分类
// @Description  创建一个新分类,返回创建后的详情
// @Tags         Admin.Category
// @Accept       json
// @Produce      json
// @Param        category  body     param.Category  true  "分类参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.CategoryDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/categories [post]
func (c *CategoryHandler) CreateCategory(ctx *gin.Context) (interface{}, error) {
	var categoryParam param.Category
	err := ctx.ShouldBindJSON(&categoryParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest)
	}
	category, err := c.CategoryService.Create(ctx, &categoryParam)
	if err != nil {
		return nil, err
	}
	return c.CategoryService.ConvertToCategoryDTO(ctx, category)
}

// UpdateCategory godoc
// @Summary      更新分类
// @Description  根据分类ID更新分类信息
// @Tags         Admin.Category
// @Accept       json
// @Produce      json
// @Param        categoryID  path     int             true  "分类ID"  example(1)
// @Param        category    body     param.Category  true  "分类参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.CategoryDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/categories/{categoryID} [put]
func (c *CategoryHandler) UpdateCategory(ctx *gin.Context) (interface{}, error) {
	var categoryParam param.Category
	err := ctx.ShouldBindJSON(&categoryParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest)
	}
	categoryID, err := util.ParamInt32(ctx, "categoryID")
	if err != nil {
		return nil, err
	}
	categoryParam.ID = categoryID
	category, err := c.CategoryService.Update(ctx, &categoryParam)
	if err != nil {
		return nil, err
	}
	return c.CategoryService.ConvertToCategoryDTO(ctx, category)
}

// UpdateCategoryBatch godoc
// @Summary      批量更新分类
// @Description  根据分类参数列表批量更新分类
// @Tags         Admin.Category
// @Accept       json
// @Produce      json
// @Param        categories  body     []param.Category  true  "分类参数列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.CategoryDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/categories/batch [put]
func (c *CategoryHandler) UpdateCategoryBatch(ctx *gin.Context) (interface{}, error) {
	categoryParams := make([]*param.Category, 0)
	err := ctx.ShouldBindJSON(&categoryParams)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	categories, err := c.CategoryService.UpdateBatch(ctx, categoryParams)
	if err != nil {
		return nil, err
	}
	return c.CategoryService.ConvertToCategoryDTOs(ctx, categories)
}

// DeleteCategory godoc
// @Summary      删除分类
// @Description  根据分类ID删除指定分类
// @Tags         Admin.Category
// @Produce      json
// @Param        categoryID  path     int  true  "分类ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/categories/{categoryID} [delete]
func (c *CategoryHandler) DeleteCategory(ctx *gin.Context) (interface{}, error) {
	categoryID, err := util.ParamInt32(ctx, "categoryID")
	if err != nil {
		return nil, err
	}
	return nil, c.CategoryService.Delete(ctx, categoryID)
}
