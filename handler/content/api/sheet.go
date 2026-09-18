package api

import (
	"html/template"

	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/handler/binding"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/service/assembler"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type SheetHandler struct {
	OptionService         service.OptionService
	SheetService          service.SheetService
	SheetCommentService   service.SheetCommentService
	SheetCommentAssembler assembler.SheetCommentAssembler
}

func NewSheetHandler(
	optionService service.OptionService,
	sheetService service.SheetService,
	sheetCommentService service.SheetCommentService,
	sheetCommentAssembler assembler.SheetCommentAssembler,
) *SheetHandler {
	return &SheetHandler{
		OptionService:         optionService,
		SheetService:          sheetService,
		SheetCommentService:   sheetCommentService,
		SheetCommentAssembler: sheetCommentAssembler,
	}
}

// ListTopComment godoc
// @Summary      查询页面的顶级评论
// @Description  分页返回指定页面下已发布的顶级评论(带是否有子评论标识)
// @Tags         Content.Sheet
// @Produce      json
// @Param        sheetID  path     int       true  "页面ID"  example(1)
// @Param        page     query     int       false  "页码(从0开始)"          example(0)
// @Param        size     query     int       false  "每页数量"              example(10)
// @Param        sort     query     []string  false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.CommentWithHasChildren}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/sheets/{sheetID}/comments/top_view [get]
func (s *SheetHandler) ListTopComment(ctx *gin.Context) (interface{}, error) {
	sheetID, err := util.ParamInt32(ctx, "sheetID")
	if err != nil {
		return nil, err
	}
	pageSize := s.OptionService.GetOrByDefault(ctx, property.CommentPageSize).(int)

	commentQuery := param.CommentQuery{}
	err = ctx.ShouldBindWith(&commentQuery, binding.CustomFormBinding)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	if commentQuery.Sort != nil && len(commentQuery.Fields) > 0 {
		commentQuery.Sort = &param.Sort{
			Fields: []string{"createTime,desc"},
		}
	}
	commentQuery.ContentID = &sheetID
	commentQuery.Keyword = nil
	commentQuery.CommentStatus = consts.CommentStatusPublished.Ptr()
	commentQuery.PageSize = pageSize
	commentQuery.ParentID = util.Int32Ptr(0)

	comments, totalCount, err := s.SheetCommentService.Page(ctx, commentQuery, consts.CommentTypeSheet)
	if err != nil {
		return nil, err
	}
	_ = s.SheetCommentAssembler.ClearSensitiveField(ctx, comments)
	commenVOs, err := s.SheetCommentAssembler.ConvertToWithHasChildren(ctx, comments)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(commenVOs, totalCount, commentQuery.Page), nil
}

// ListChildren godoc
// @Summary      查询页面评论的子评论
// @Description  返回指定页面下某条评论的全部子评论
// @Tags         Content.Sheet
// @Produce      json
// @Param        sheetID   path     int  true  "页面ID"     example(1)
// @Param        parentID  path     int  true  "父评论ID"  example(1)
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/sheets/{sheetID}/comments/{parentID}/children [get]
func (s *SheetHandler) ListChildren(ctx *gin.Context) (interface{}, error) {
	sheetID, err := util.ParamInt32(ctx, "sheetID")
	if err != nil {
		return nil, err
	}
	parentID, err := util.ParamInt32(ctx, "parentID")
	if err != nil {
		return nil, err
	}
	children, err := s.SheetCommentService.GetChildren(ctx, parentID, sheetID, consts.CommentTypeSheet)
	if err != nil {
		return nil, err
	}
	_ = s.SheetCommentAssembler.ClearSensitiveField(ctx, children)
	return s.SheetCommentAssembler.ConvertToDTOList(ctx, children)
}

// ListCommentTree godoc
// @Summary      查询页面评论的树形视图
// @Description  分页返回指定页面下全部已发布评论并按父子关系组织为树形结构
// @Tags         Content.Sheet
// @Produce      json
// @Param        sheetID  path     int       true  "页面ID"  example(1)
// @Param        page     query     int       false  "页码(从0开始)"          example(0)
// @Param        size     query     int       false  "每页数量"              example(10)
// @Param        sort     query     []string  false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.Comment}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/sheets/{sheetID}/comments/tree_view [get]
func (s *SheetHandler) ListCommentTree(ctx *gin.Context) (interface{}, error) {
	sheetID, err := util.ParamInt32(ctx, "sheetID")
	if err != nil {
		return nil, err
	}
	pageSize := s.OptionService.GetOrByDefault(ctx, property.CommentPageSize).(int)

	commentQuery := param.CommentQuery{}
	err = ctx.ShouldBindWith(&commentQuery, binding.CustomFormBinding)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	if commentQuery.Sort != nil && len(commentQuery.Fields) > 0 {
		commentQuery.Sort = &param.Sort{
			Fields: []string{"createTime,desc"},
		}
	}
	commentQuery.ContentID = &sheetID
	commentQuery.Keyword = nil
	commentQuery.CommentStatus = consts.CommentStatusPublished.Ptr()
	commentQuery.PageSize = pageSize
	commentQuery.ParentID = util.Int32Ptr(0)

	allComments, err := s.SheetCommentService.GetByContentID(ctx, sheetID, consts.CommentTypeSheet, commentQuery.Sort)
	if err != nil {
		return nil, err
	}
	_ = s.SheetCommentAssembler.ClearSensitiveField(ctx, allComments)
	commentVOs, total, err := s.SheetCommentAssembler.PageConvertToVOs(ctx, allComments, commentQuery.Page)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(commentVOs, total, commentQuery.Page), nil
}

// ListComment godoc
// @Summary      查询页面评论的列表视图
// @Description  分页返回指定页面下已发布评论,每条评论携带其父评论信息
// @Tags         Content.Sheet
// @Produce      json
// @Param        sheetID  path     int       true  "页面ID"  example(1)
// @Param        page     query     int       false  "页码(从0开始)"          example(0)
// @Param        size     query     int       false  "每页数量"              example(10)
// @Param        sort     query     []string  false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.CommentWithParent}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/sheets/{sheetID}/comments/list_view [get]
func (s *SheetHandler) ListComment(ctx *gin.Context) (interface{}, error) {
	sheetID, err := util.ParamInt32(ctx, "sheetID")
	if err != nil {
		return nil, err
	}
	pageSize := s.OptionService.GetOrByDefault(ctx, property.CommentPageSize).(int)

	commentQuery := param.CommentQuery{}
	err = ctx.ShouldBindWith(&commentQuery, binding.CustomFormBinding)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	if commentQuery.Sort != nil && len(commentQuery.Fields) > 0 {
		commentQuery.Sort = &param.Sort{
			Fields: []string{"createTime,desc"},
		}
	}
	commentQuery.ContentID = &sheetID
	commentQuery.Keyword = nil
	commentQuery.CommentStatus = consts.CommentStatusPublished.Ptr()
	commentQuery.PageSize = pageSize
	commentQuery.ParentID = util.Int32Ptr(0)

	comments, total, err := s.SheetCommentService.Page(ctx, commentQuery, consts.CommentTypeSheet)
	if err != nil {
		return nil, err
	}
	_ = s.SheetCommentAssembler.ClearSensitiveField(ctx, comments)
	result, err := s.SheetCommentAssembler.ConvertToWithParentVO(ctx, comments)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(result, total, commentQuery.Page), nil
}

// CreateComment godoc
// @Summary      创建页面评论
// @Description  为指定页面创建一条新评论,作者、邮箱、内容会做 HTML 转义
// @Tags         Content.Sheet
// @Accept       json
// @Produce      json
// @Param        comment  body     param.Comment  true  "评论参数"
// @Success      200  {object}  dto.BaseDTO{data=dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/sheets/comments [post]
func (s *SheetHandler) CreateComment(ctx *gin.Context) (interface{}, error) {
	comment := param.Comment{}
	err := ctx.ShouldBindJSON(&comment)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	if comment.AuthorURL != "" {
		err = util.Validate.Var(comment.AuthorURL, "http_url")
		if err != nil {
			return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
		}
	}
	comment.Author = template.HTMLEscapeString(comment.Author)
	comment.AuthorURL = template.HTMLEscapeString(comment.AuthorURL)
	comment.Content = template.HTMLEscapeString(comment.Content)
	comment.Email = template.HTMLEscapeString(comment.Email)
	comment.CommentType = consts.CommentTypeSheet
	result, err := s.SheetCommentService.CreateBy(ctx, &comment)
	if err != nil {
		return nil, err
	}
	return s.SheetCommentAssembler.ConvertToDTO(ctx, result)
}
