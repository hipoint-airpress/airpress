package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/handler/binding"
	"github.com/hipoint-airpress/airpress/handler/trans"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/service/assembler"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type PostHandler struct {
	PostService   service.PostService
	PostAssembler assembler.PostAssembler
}

func NewPostHandler(postService service.PostService, postAssembler assembler.PostAssembler) *PostHandler {
	return &PostHandler{
		PostService:   postService,
		PostAssembler: postAssembler,
	}
}

// ListPosts godoc
// @Summary      分页查询文章列表
// @Description  支持按关键词、状态、分类、标签过滤,支持排序与分页
// @Tags         Admin.Post
// @Accept       json
// @Produce      json
// @Param        page        query     int      false  "页码(从0开始)"          example(0)
// @Param        size        query     int      false  "每页数量"              example(10)
// @Param        sort        query     []string false  "排序字段,如 createTime,desc" collectionFormat(multi)
// @Param        keyword     query     string   false  "标题关键词"
// @Param        statuses    query     []int    false  "状态过滤"            collectionFormat(multi)
// @Param        categoryId  query     int      false  "分类ID"
// @Param        tagId       query     int      false  "标签ID"
// @Param        more        query     bool     false  "true返回详情VO,false返回精简DTO"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]dto.Post}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/posts [get]
func (p *PostHandler) ListPosts(ctx *gin.Context) (interface{}, error) {
	postQuery := param.PostQuery{}
	err := ctx.ShouldBindWith(&postQuery, binding.CustomFormBinding)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	if postQuery.Sort == nil {
		postQuery.Sort = &param.Sort{Fields: []string{"topPriority,desc", "createTime,desc"}}
	}
	posts, totalCount, err := p.PostService.Page(ctx, postQuery)
	if err != nil {
		return nil, err
	}
	if postQuery.More == nil || *postQuery.More {
		postVOs, err := p.PostAssembler.ConvertToListVO(ctx, posts)
		return dto.NewPage(postVOs, totalCount, postQuery.Page), err
	}
	postDTOs := make([]*dto.Post, 0)
	for _, post := range posts {
		postDTO, err := p.PostAssembler.ConvertToSimpleDTO(ctx, post)
		if err != nil {
			return nil, err
		}
		postDTOs = append(postDTOs, postDTO)
	}
	return dto.NewPage(postDTOs, totalCount, postQuery.Page), nil
}

func (p *PostHandler) ListLatestPosts(ctx *gin.Context) (interface{}, error) {
	top, err := util.MustGetQueryInt32(ctx, "top")
	if err != nil {
		top = 10
	}
	postQuery := param.PostQuery{
		Page: param.Page{
			PageSize: int(top),
			PageNum:  0,
		},
		Sort: &param.Sort{
			Fields: []string{"createTime,desc"},
		},
		Keyword:    nil,
		CategoryID: nil,
		More:       util.BoolPtr(false),
	}
	posts, _, err := p.PostService.Page(ctx, postQuery)
	if err != nil {
		return nil, err
	}
	postMinimals := make([]*dto.PostMinimal, 0, len(posts))

	for _, post := range posts {
		postMinimal, err := p.PostAssembler.ConvertToMinimalDTO(ctx, post)
		if err != nil {
			return nil, err
		}
		postMinimals = append(postMinimals, postMinimal)
	}
	return postMinimals, nil
}

func (p *PostHandler) ListPostsByStatus(ctx *gin.Context) (interface{}, error) {
	var postQuery param.PostQuery
	err := ctx.ShouldBindWith(&postQuery, binding.CustomFormBinding)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	if postQuery.Sort == nil {
		postQuery.Sort = &param.Sort{Fields: []string{"createTime,desc"}}
	}

	status, err := util.ParamInt32(ctx, "status")
	if err != nil {
		return nil, err
	}
	postQuery.Statuses = make([]*consts.PostStatus, 0)
	statusType := consts.PostStatus(status)
	postQuery.Statuses = append(postQuery.Statuses, &statusType)

	posts, totalCount, err := p.PostService.Page(ctx, postQuery)
	if err != nil {
		return nil, err
	}
	if postQuery.More == nil {
		*postQuery.More = false
	}
	if postQuery.More == nil {
		postVOs, err := p.PostAssembler.ConvertToListVO(ctx, posts)
		return dto.NewPage(postVOs, totalCount, postQuery.Page), err
	}

	postDTOs := make([]*dto.Post, 0)
	for _, post := range posts {
		postDTO, err := p.PostAssembler.ConvertToSimpleDTO(ctx, post)
		if err != nil {
			return nil, err
		}
		postDTOs = append(postDTOs, postDTO)
	}

	return dto.NewPage(postDTOs, totalCount, postQuery.Page), nil
}

// GetByPostID godoc
// @Summary      根据文章ID获取详情
// @Description  返回文章详情,包含正文、评论数等
// @Tags         Admin.Post
// @Produce      json
// @Param        postID  path     int  true  "文章ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=vo.PostDetailVO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/posts/{postID} [get]
func (p *PostHandler) GetByPostID(ctx *gin.Context) (interface{}, error) {
	postIDStr := ctx.Param("postID")
	postID, err := strconv.ParseInt(postIDStr, 10, 32)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	post, err := p.PostService.GetByPostID(ctx, int32(postID))
	if err != nil {
		return nil, err
	}
	postDetailVO, err := p.PostAssembler.ConvertToDetailVO(ctx, post)
	if err != nil {
		return nil, err
	}
	return postDetailVO, nil
}

// CreatePost godoc
// @Summary      创建文章
// @Description  创建一篇新文章,返回创建后的详情
// @Tags         Admin.Post
// @Accept       json
// @Produce      json
// @Param        post  body     param.Post  true  "文章参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=vo.PostDetailVO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/posts [post]
func (p *PostHandler) CreatePost(ctx *gin.Context) (interface{}, error) {
	var postParam param.Post
	err := ctx.ShouldBindJSON(&postParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}

	post, err := p.PostService.Create(ctx, &postParam)
	if err != nil {
		return nil, err
	}
	return p.PostAssembler.ConvertToDetailVO(ctx, post)
}

// UpdatePost godoc
// @Summary      更新文章
// @Description  根据文章ID更新文章内容
// @Tags         Admin.Post
// @Accept       json
// @Produce      json
// @Param        postID  path     int          true  "文章ID"  example(1)
// @Param        post    body     param.Post   true  "文章参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=vo.PostDetailVO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/posts/{postID} [put]
func (p *PostHandler) UpdatePost(ctx *gin.Context) (interface{}, error) {
	var postParam param.Post
	err := ctx.ShouldBindJSON(&postParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}

	postIDStr := ctx.Param("postID")
	postID, err := strconv.ParseInt(postIDStr, 10, 32)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}

	postDetailVO, err := p.PostService.Update(ctx, int32(postID), &postParam)
	if err != nil {
		return nil, err
	}
	return postDetailVO, nil
}

func (p *PostHandler) UpdatePostStatus(ctx *gin.Context) (interface{}, error) {
	postIDStr := ctx.Param("postID")
	postID, err := strconv.ParseInt(postIDStr, 10, 32)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	statusStr, err := util.ParamString(ctx, "status")
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	status, err := consts.PostStatusFromString(statusStr)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	if int32(status) < int32(consts.PostStatusPublished) || int32(status) > int32(consts.PostStatusIntimate) {
		return nil, xerr.WithStatus(nil, xerr.StatusBadRequest).WithMsg("status error")
	}
	post, err := p.PostService.UpdateStatus(ctx, int32(postID), status)
	if err != nil {
		return nil, err
	}
	return p.PostAssembler.ConvertToMinimalDTO(ctx, post)
}

func (p *PostHandler) UpdatePostStatusBatch(ctx *gin.Context) (interface{}, error) {
	statusStr, err := util.ParamString(ctx, "status")
	if err != nil {
		return nil, err
	}
	status, err := consts.PostStatusFromString(statusStr)
	if err != nil {
		return nil, err
	}
	if int32(status) < int32(consts.PostStatusPublished) || int32(status) > int32(consts.PostStatusIntimate) {
		return nil, xerr.WithStatus(nil, xerr.StatusBadRequest).WithMsg("status error")
	}
	ids := make([]int32, 0)
	err = ctx.ShouldBind(&ids)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("post ids error")
	}

	return p.PostService.UpdateStatusBatch(ctx, status, ids)
}

func (p *PostHandler) UpdatePostDraft(ctx *gin.Context) (interface{}, error) {
	postID, err := util.ParamInt32(ctx, "postID")
	if err != nil {
		return nil, err
	}
	var postContentParam param.PostContent
	err = ctx.ShouldBindJSON(&postContentParam)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("content param error")
	}
	post, err := p.PostService.UpdateDraftContent(ctx, postID, postContentParam.Content, postContentParam.OriginalContent)
	if err != nil {
		return nil, err
	}
	return p.PostAssembler.ConvertToDetailDTO(ctx, post)
}

// DeletePost godoc
// @Summary      删除文章
// @Description  根据文章ID删除指定文章
// @Tags         Admin.Post
// @Produce      json
// @Param        postID  path     int  true  "文章ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/posts/{postID} [delete]
func (p *PostHandler) DeletePost(ctx *gin.Context) (interface{}, error) {
	postID, err := util.ParamInt32(ctx, "postID")
	if err != nil {
		return nil, err
	}
	return nil, p.PostService.Delete(ctx, postID)
}

func (p *PostHandler) DeletePostBatch(ctx *gin.Context) (interface{}, error) {
	postIDs := make([]int32, 0)
	err := ctx.ShouldBind(&postIDs)
	if err != nil {
		return nil, xerr.WithMsg(err, "postIDs error").WithStatus(xerr.StatusBadRequest)
	}
	return nil, p.PostService.DeleteBatch(ctx, postIDs)
}

func (p *PostHandler) PreviewPost(ctx *gin.Context) {
	postID, err := util.ParamInt32(ctx, "postID")
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		_ = ctx.Error(err)
		return
	}
	previewPath, err := p.PostService.Preview(ctx, postID)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		_ = ctx.Error(err)
		return
	}
	ctx.String(http.StatusOK, previewPath)
}
