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

type TagHandler struct {
	PostTagService service.PostTagService
	TagService     service.TagService
}

func NewTagHandler(postTagService service.PostTagService, tagService service.TagService) *TagHandler {
	return &TagHandler{
		PostTagService: postTagService,
		TagService:     tagService,
	}
}

// ListTags godoc
// @Summary      查询所有标签
// @Description  返回所有标签列表,more=true 时返回带文章数的详情列表
// @Tags         Admin.Tag
// @Accept       json
// @Produce      json
// @Param        sort  query     []string  false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Param        more  query     bool      false  "true返回带文章数的详情"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Tag}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/tags [get]
func (t *TagHandler) ListTags(ctx *gin.Context) (interface{}, error) {
	sort := param.Sort{}
	err := ctx.ShouldBindQuery(&sort)
	if err != nil {
		return nil, xerr.WithMsg(err, "sort parameter error").WithStatus(xerr.StatusBadRequest)
	}
	if len(sort.Fields) == 0 {
		sort.Fields = append(sort.Fields, "createTime,desc")
	}
	more, _ := util.MustGetQueryBool(ctx, "more")
	if more {
		return t.PostTagService.ListAllTagWithPostCount(ctx, &sort)
	}
	tags, err := t.TagService.ListAll(ctx, &sort)
	if err != nil {
		return nil, err
	}
	return t.TagService.ConvertToDTOs(ctx, tags)
}

// GetTagByID godoc
// @Summary      根据ID获取标签
// @Description  返回指定 ID 的标签详情
// @Tags         Admin.Tag
// @Produce      json
// @Param        id  path     int  true  "标签ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Tag}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/tags/{id} [get]
func (t *TagHandler) GetTagByID(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	tag, err := t.TagService.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return t.TagService.ConvertToDTO(ctx, tag)
}

// CreateTag godoc
// @Summary      创建标签
// @Description  创建一个新标签,返回创建后的详情
// @Tags         Admin.Tag
// @Accept       json
// @Produce      json
// @Param        tag  body     param.Tag  true  "标签参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Tag}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/tags [post]
func (t *TagHandler) CreateTag(ctx *gin.Context) (interface{}, error) {
	tagParam := &param.Tag{}
	err := ctx.ShouldBindJSON(tagParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	tag, err := t.TagService.Create(ctx, tagParam)
	if err != nil {
		return nil, err
	}
	return t.TagService.ConvertToDTO(ctx, tag)
}

// UpdateTag godoc
// @Summary      更新标签
// @Description  根据标签ID更新标签信息
// @Tags         Admin.Tag
// @Accept       json
// @Produce      json
// @Param        id   path     int        true  "标签ID"  example(1)
// @Param        tag  body     param.Tag  true  "标签参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Tag}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/tags/{id} [put]
func (t *TagHandler) UpdateTag(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	tagParam := &param.Tag{}
	err = ctx.ShouldBindJSON(tagParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	tag, err := t.TagService.Update(ctx, id, tagParam)
	if err != nil {
		return nil, err
	}
	return t.TagService.ConvertToDTO(ctx, tag)
}

// DeleteTag godoc
// @Summary      删除标签
// @Description  根据标签ID删除指定标签
// @Tags         Admin.Tag
// @Produce      json
// @Param        id  path     int  true  "标签ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/tags/{id} [delete]
func (t *TagHandler) DeleteTag(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	return nil, t.TagService.Delete(ctx, id)
}
