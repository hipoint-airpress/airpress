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

type PostHandler struct {
	OptionService        service.OptionService
	PostService          service.PostService
	PostCommentService   service.PostCommentService
	PostCommentAssembler assembler.PostCommentAssembler
}

func NewPostHandler(
	optionService service.OptionService,
	postService service.PostService,
	postCommentService service.PostCommentService,
	postCommentAssembler assembler.PostCommentAssembler,
) *PostHandler {
	return &PostHandler{
		OptionService:        optionService,
		PostService:          postService,
		PostCommentService:   postCommentService,
		PostCommentAssembler: postCommentAssembler,
	}
}

// ListTopComment godoc
// @Summary      查询文章的顶级评论
// @Description  分页返回指定文章下已发布的顶级评论(带是否有子评论标识)
// @Tags         Content.Post
// @Produce      json
// @Param        postID  path     int       true  "文章ID"  example(1)
// @Param        page    query     int       false  "页码(从0开始)"          example(0)
// @Param        size    query     int       false  "每页数量"              example(10)
// @Param        sort    query     []string  false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.CommentWithHasChildren}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/posts/{postID}/comments/top_view [get]
func (p *PostHandler) ListTopComment(ctx *gin.Context) (interface{}, error) {
	postID, err := util.ParamInt32(ctx, "postID")
	if err != nil {
		return nil, err
	}
	pageSize := p.OptionService.GetOrByDefault(ctx, property.CommentPageSize).(int)

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
	commentQuery.ContentID = &postID
	commentQuery.Keyword = nil
	commentQuery.CommentStatus = consts.CommentStatusPublished.Ptr()
	commentQuery.PageSize = pageSize
	commentQuery.ParentID = util.Int32Ptr(0)

	comments, totalCount, err := p.PostCommentService.Page(ctx, commentQuery, consts.CommentTypePost)
	if err != nil {
		return nil, err
	}
	_ = p.PostCommentAssembler.ClearSensitiveField(ctx, comments)
	commenVOs, err := p.PostCommentAssembler.ConvertToWithHasChildren(ctx, comments)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(commenVOs, totalCount, commentQuery.Page), nil
}

// ListChildren godoc
// @Summary      查询文章评论的子评论
// @Description  返回指定文章下某条评论的全部子评论
// @Tags         Content.Post
// @Produce      json
// @Param        postID    path     int  true  "文章ID"     example(1)
// @Param        parentID  path     int  true  "父评论ID"  example(1)
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/posts/{postID}/comments/{parentID}/children [get]
func (p *PostHandler) ListChildren(ctx *gin.Context) (interface{}, error) {
	postID, err := util.ParamInt32(ctx, "postID")
	if err != nil {
		return nil, err
	}
	parentID, err := util.ParamInt32(ctx, "parentID")
	if err != nil {
		return nil, err
	}
	children, err := p.PostCommentService.GetChildren(ctx, parentID, postID, consts.CommentTypePost)
	if err != nil {
		return nil, err
	}
	_ = p.PostCommentAssembler.ClearSensitiveField(ctx, children)
	return p.PostCommentAssembler.ConvertToDTOList(ctx, children)
}

// ListCommentTree godoc
// @Summary      查询文章评论的树形视图
// @Description  分页返回指定文章下全部已发布评论并按父子关系组织为树形结构
// @Tags         Content.Post
// @Produce      json
// @Param        postID  path     int       true  "文章ID"  example(1)
// @Param        page    query     int       false  "页码(从0开始)"          example(0)
// @Param        size    query     int       false  "每页数量"              example(10)
// @Param        sort    query     []string  false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.Comment}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/posts/{postID}/comments/tree_view [get]
func (p *PostHandler) ListCommentTree(ctx *gin.Context) (interface{}, error) {
	postID, err := util.ParamInt32(ctx, "postID")
	if err != nil {
		return nil, err
	}
	pageSize := p.OptionService.GetOrByDefault(ctx, property.CommentPageSize).(int)

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
	commentQuery.ContentID = &postID
	commentQuery.Keyword = nil
	commentQuery.CommentStatus = consts.CommentStatusPublished.Ptr()
	commentQuery.PageSize = pageSize
	commentQuery.ParentID = util.Int32Ptr(0)

	allComments, err := p.PostCommentService.GetByContentID(ctx, postID, consts.CommentTypePost, commentQuery.Sort)
	if err != nil {
		return nil, err
	}
	_ = p.PostCommentAssembler.ClearSensitiveField(ctx, allComments)
	commentVOs, total, err := p.PostCommentAssembler.PageConvertToVOs(ctx, allComments, commentQuery.Page)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(commentVOs, total, commentQuery.Page), nil
}

// ListComment godoc
// @Summary      查询文章评论的列表视图
// @Description  分页返回指定文章下已发布评论,每条评论携带其父评论信息
// @Tags         Content.Post
// @Produce      json
// @Param        postID  path     int       true  "文章ID"  example(1)
// @Param        page    query     int       false  "页码(从0开始)"          example(0)
// @Param        size    query     int       false  "每页数量"              example(10)
// @Param        sort    query     []string  false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.CommentWithParent}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/posts/{postID}/comments/list_view [get]
func (p *PostHandler) ListComment(ctx *gin.Context) (interface{}, error) {
	postID, err := util.ParamInt32(ctx, "postID")
	if err != nil {
		return nil, err
	}
	pageSize := p.OptionService.GetOrByDefault(ctx, property.CommentPageSize).(int)

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
	commentQuery.ContentID = &postID
	commentQuery.Keyword = nil
	commentQuery.CommentStatus = consts.CommentStatusPublished.Ptr()
	commentQuery.PageSize = pageSize
	commentQuery.ParentID = util.Int32Ptr(0)

	comments, total, err := p.PostCommentService.Page(ctx, commentQuery, consts.CommentTypePost)
	if err != nil {
		return nil, err
	}
	_ = p.PostCommentAssembler.ClearSensitiveField(ctx, comments)
	result, err := p.PostCommentAssembler.ConvertToWithParentVO(ctx, comments)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(result, total, commentQuery.Page), nil
}

// CreateComment godoc
// @Summary      创建文章评论
// @Description  为指定文章创建一条新评论,作者、邮箱、内容会做 HTML 转义
// @Tags         Content.Post
// @Accept       json
// @Produce      json
// @Param        comment  body     param.Comment  true  "评论参数"
// @Success      200  {object}  dto.BaseDTO{data=dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/posts/comments [post]
func (p *PostHandler) CreateComment(ctx *gin.Context) (interface{}, error) {
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
	comment.CommentType = consts.CommentTypePost
	result, err := p.PostCommentService.CreateBy(ctx, &comment)
	if err != nil {
		return nil, err
	}
	return p.PostCommentAssembler.ConvertToDTO(ctx, result)
}

// Like godoc
// @Summary      文章点赞
// @Description  为指定文章点赞,点赞数 +1
// @Tags         Content.Post
// @Produce      json
// @Param        postID  path     int  true  "文章ID"  example(1)
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/posts/{postID}/likes [post]
func (p *PostHandler) Like(ctx *gin.Context) (interface{}, error) {
	postID, err := util.ParamInt32(ctx, "postID")
	if err != nil {
		return nil, err
	}
	return nil, p.PostService.IncreaseLike(ctx, postID)
}
