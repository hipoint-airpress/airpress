package admin

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/handler/binding"
	"github.com/hipoint-airpress/airpress/handler/trans"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/service/assembler"
	"github.com/hipoint-airpress/airpress/service/impl"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type PostCommentHandler struct {
	PostCommentService   service.PostCommentService
	OptionService        service.OptionService
	PostService          service.PostService
	PostAssembler        assembler.PostAssembler
	PostCommentAssembler assembler.PostCommentAssembler
}

func NewPostCommentHandler(
	postCommentHandler service.PostCommentService,
	optionService service.OptionService,
	postService service.PostService,
	postAssembler assembler.PostAssembler,
	postCommentAssembler assembler.PostCommentAssembler,
) *PostCommentHandler {
	return &PostCommentHandler{
		PostCommentService:   postCommentHandler,
		OptionService:        optionService,
		PostService:          postService,
		PostAssembler:        postAssembler,
		PostCommentAssembler: postCommentAssembler,
	}
}

// ListPostComment godoc
// @Summary      分页查询文章评论列表
// @Description  支持按关键词、状态过滤,支持排序与分页
// @Tags         Admin.Comment.Post
// @Accept       json
// @Produce      json
// @Param        page      query     int        false  "页码(从0开始)"  example(0)
// @Param        size      query     int        false  "每页数量"        example(10)
// @Param        sort      query     []string   false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Param        keyword   query     string     false  "评论关键词"
// @Param        status    query     []string   false  "状态过滤"      collectionFormat(multi)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.PostCommentWithPost}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/posts/comments [get]
func (p *PostCommentHandler) ListPostComment(ctx *gin.Context) (interface{}, error) {
	var commentQuery param.CommentQuery
	err := ctx.ShouldBindWith(&commentQuery, binding.CustomFormBinding)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	commentQuery.Sort = &param.Sort{
		Fields: []string{"createTime,desc"},
	}
	comments, totalCount, err := p.PostCommentService.Page(ctx, commentQuery, consts.CommentTypePost)
	if err != nil {
		return nil, err
	}
	commentDTOs, err := p.PostCommentAssembler.ConvertToWithPost(ctx, comments)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(commentDTOs, totalCount, commentQuery.Page), nil
}

// ListPostCommentLatest godoc
// @Summary      查询最新文章评论
// @Description  返回指定数量的最新文章评论列表
// @Tags         Admin.Comment.Post
// @Produce      json
// @Param        top  query     int  false  "返回数量"  example(10)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]vo.PostCommentWithPost}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/posts/comments/latest [get]
func (p *PostCommentHandler) ListPostCommentLatest(ctx *gin.Context) (interface{}, error) {
	top, err := util.MustGetQueryInt32(ctx, "top")
	if err != nil {
		return nil, err
	}
	commentQuery := param.CommentQuery{
		Sort: &param.Sort{Fields: []string{"createTime,desc"}},
		Page: param.Page{PageNum: 0, PageSize: int(top)},
	}
	comments, _, err := p.PostCommentService.Page(ctx, commentQuery, consts.CommentTypePost)
	if err != nil {
		return nil, err
	}
	return p.PostCommentAssembler.ConvertToWithPost(ctx, comments)
}

// ListPostCommentAsTree godoc
// @Summary      树形文章评论列表
// @Description  根据文章ID返回评论的树形结构,支持分页
// @Tags         Admin.Comment.Post
// @Produce      json
// @Param        postID  path     int  true  "文章ID"  example(1)
// @Param        page    query     int  false  "页码(从0开始)"  example(0)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.Comment}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/posts/comments/{postID}/tree_view [get]
func (p *PostCommentHandler) ListPostCommentAsTree(ctx *gin.Context) (interface{}, error) {
	postID, err := util.ParamInt32(ctx, "postID")
	if err != nil {
		return nil, err
	}
	pageNum, err := util.MustGetQueryInt32(ctx, "page")
	if err != nil {
		return nil, err
	}
	pageSize, err := p.OptionService.GetOrByDefaultWithErr(ctx, property.CommentPageSize, property.CommentPageSize.DefaultValue)
	if err != nil {
		return nil, err
	}
	page := param.Page{PageSize: pageSize.(int), PageNum: int(pageNum)}
	allComments, err := p.PostCommentService.GetByContentID(ctx, postID, consts.CommentTypePost, &param.Sort{Fields: []string{"createTime,desc"}})
	if err != nil {
		return nil, err
	}
	commentVOs, totalCount, err := p.PostCommentAssembler.PageConvertToVOs(ctx, allComments, page)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(commentVOs, totalCount, page), nil
}

// ListPostCommentWithParent godoc
// @Summary      带父评论的文章评论列表
// @Description  根据文章ID返回评论列表(每条带父评论信息),支持分页
// @Tags         Admin.Comment.Post
// @Produce      json
// @Param        postID  path     int  true  "文章ID"  example(1)
// @Param        page    query     int  false  "页码(从0开始)"  example(0)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.CommentWithParent}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/posts/comments/{postID}/list_view [get]
func (p *PostCommentHandler) ListPostCommentWithParent(ctx *gin.Context) (interface{}, error) {
	postID, err := util.ParamInt32(ctx, "postID")
	if err != nil {
		return nil, err
	}
	pageNum, err := util.MustGetQueryInt32(ctx, "page")
	if err != nil {
		return nil, err
	}

	pageSize, err := p.OptionService.GetOrByDefaultWithErr(ctx, property.CommentPageSize, property.CommentPageSize.DefaultValue)
	if err != nil {
		return nil, err
	}

	page := param.Page{PageNum: int(pageNum), PageSize: pageSize.(int)}

	comments, totalCount, err := p.PostCommentService.Page(ctx, param.CommentQuery{
		ContentID: &postID,
		Page:      page,
		Sort:      &param.Sort{Fields: []string{"createTime,desc"}},
	}, consts.CommentTypePost)
	if err != nil {
		return nil, err
	}

	commentsWithParent, err := p.PostCommentAssembler.ConvertToWithParentVO(ctx, comments)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(commentsWithParent, totalCount, page), nil
}

// CreatePostComment godoc
// @Summary      创建文章评论
// @Description  在指定文章下创建一条评论
// @Tags         Admin.Comment.Post
// @Accept       json
// @Produce      json
// @Param        comment  body     param.AdminComment  true  "评论参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/posts/comments [post]
func (p *PostCommentHandler) CreatePostComment(ctx *gin.Context) (interface{}, error) {
	var commentParam *param.AdminComment
	err := ctx.ShouldBindJSON(&commentParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	user, err := impl.MustGetAuthorizedUser(ctx)
	if err != nil || user == nil {
		return nil, err
	}
	blogURL, err := p.OptionService.GetBlogBaseURL(ctx)
	if err != nil {
		return nil, err
	}
	commonParam := param.Comment{
		Author:            user.Username,
		Email:             user.Email,
		AuthorURL:         blogURL,
		Content:           commentParam.Content,
		PostID:            commentParam.PostID,
		ParentID:          commentParam.ParentID,
		AllowNotification: true,
		CommentType:       consts.CommentTypePost,
	}
	comment, err := p.PostCommentService.CreateBy(ctx, &commonParam)
	if err != nil {
		return nil, err
	}
	return p.PostCommentAssembler.ConvertToDTO(ctx, comment)
}

// UpdatePostComment godoc
// @Summary      更新文章评论
// @Description  根据评论ID更新评论内容
// @Tags         Admin.Comment.Post
// @Accept       json
// @Produce      json
// @Param        commentID  path     int             true  "评论ID"  example(1)
// @Param        comment    body     param.Comment   true  "评论参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/posts/comments/{commentID} [put]
func (p *PostCommentHandler) UpdatePostComment(ctx *gin.Context) (interface{}, error) {
	commentID, err := util.ParamInt32(ctx, "commentID")
	if err != nil {
		return nil, err
	}
	var commentParam *param.Comment
	err = ctx.ShouldBindJSON(&commentParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	if commentParam.AuthorURL != "" {
		err = util.Validate.Var(commentParam.AuthorURL, "url")
		if err != nil {
			return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("url is not available")
		}
	}
	comment, err := p.PostCommentService.UpdateBy(ctx, commentID, commentParam)
	if err != nil {
		return nil, err
	}

	return p.PostCommentAssembler.ConvertToDTO(ctx, comment)
}

// UpdatePostCommentStatus godoc
// @Summary      更新文章评论状态
// @Description  根据评论ID和状态更新评论状态
// @Tags         Admin.Comment.Post
// @Produce      json
// @Param        commentID  path     int     true  "评论ID"  example(1)
// @Param        status     path     string  true  "状态值"  example(published)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/posts/comments/{commentID}/status/{status} [put]
func (p *PostCommentHandler) UpdatePostCommentStatus(ctx *gin.Context) (interface{}, error) {
	commentID, err := util.ParamInt32(ctx, "commentID")
	if err != nil {
		return nil, err
	}
	strStatus, err := util.ParamString(ctx, "status")
	if err != nil {
		return nil, err
	}
	status, err := consts.CommentStatusFromString(strStatus)
	if err != nil {
		return nil, err
	}
	return p.PostCommentService.UpdateStatus(ctx, commentID, status)
}

// UpdatePostCommentStatusBatch godoc
// @Summary      批量更新文章评论状态
// @Description  根据评论ID列表和状态批量更新评论状态
// @Tags         Admin.Comment.Post
// @Accept       json
// @Produce      json
// @Param        status  path     string  true  "状态值"  example(published)
// @Param        ids     body     []int   true  "评论ID列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/posts/comments/status/{status} [put]
func (p *PostCommentHandler) UpdatePostCommentStatusBatch(ctx *gin.Context) (interface{}, error) {
	strStatus, err := util.ParamString(ctx, "status")
	if err != nil {
		return nil, err
	}
	status, err := consts.CommentStatusFromString(strStatus)
	if err != nil {
		return nil, err
	}

	ids := make([]int32, 0)
	err = ctx.ShouldBindJSON(&ids)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("post ids error")
	}
	comments, err := p.PostCommentService.UpdateStatusBatch(ctx, ids, status)
	if err != nil {
		return nil, err
	}
	return p.PostCommentAssembler.ConvertToDTOList(ctx, comments)
}

// DeletePostComment godoc
// @Summary      删除文章评论
// @Description  根据评论ID删除指定文章评论
// @Tags         Admin.Comment.Post
// @Produce      json
// @Param        commentID  path     int  true  "评论ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/posts/comments/{commentID} [delete]
func (p *PostCommentHandler) DeletePostComment(ctx *gin.Context) (interface{}, error) {
	commentID, err := util.ParamInt32(ctx, "commentID")
	if err != nil {
		return nil, err
	}
	return nil, p.PostCommentService.Delete(ctx, commentID)
}

// DeletePostCommentBatch godoc
// @Summary      批量删除文章评论
// @Description  根据评论ID列表批量删除文章评论
// @Tags         Admin.Comment.Post
// @Accept       json
// @Produce      json
// @Param        ids  body     []int  true  "评论ID列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/posts/comments [delete]
func (p *PostCommentHandler) DeletePostCommentBatch(ctx *gin.Context) (interface{}, error) {
	ids := make([]int32, 0)
	err := ctx.ShouldBindJSON(&ids)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("post ids error")
	}
	return nil, p.PostCommentService.DeleteBatch(ctx, ids)
}
