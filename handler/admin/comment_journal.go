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

type JournalCommentHandler struct {
	JournalCommentService   service.JournalCommentService
	OptionService           service.OptionService
	JournalService          service.JournalService
	JournalCommentAssembler assembler.JournalCommentAssembler
}

func NewJournalCommentHandler(journalCommentService service.JournalCommentService, optionService service.OptionService, journalService service.JournalService, journalCommentAssembler assembler.JournalCommentAssembler) *JournalCommentHandler {
	return &JournalCommentHandler{
		JournalCommentService:   journalCommentService,
		OptionService:           optionService,
		JournalService:          journalService,
		JournalCommentAssembler: journalCommentAssembler,
	}
}

// ListJournalComment godoc
// @Summary      分页查询日志评论列表
// @Description  支持按关键词、状态过滤,支持排序与分页
// @Tags         Admin.Comment.Journal
// @Accept       json
// @Produce      json
// @Param        page      query     int        false  "页码(从0开始)"  example(0)
// @Param        size      query     int        false  "每页数量"        example(10)
// @Param        sort      query     []string   false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Param        keyword   query     string     false  "评论关键词"
// @Param        status    query     []string   false  "状态过滤"      collectionFormat(multi)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.JournalCommentWithJournal}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/journals/comments [get]
func (j *JournalCommentHandler) ListJournalComment(ctx *gin.Context) (interface{}, error) {
	var commentQuery param.CommentQuery
	err := ctx.ShouldBindWith(&commentQuery, binding.CustomFormBinding)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	commentQuery.Sort = &param.Sort{
		Fields: []string{"createTime,desc"},
	}
	comments, totalCount, err := j.JournalCommentService.Page(ctx, commentQuery, consts.CommentTypeJournal)
	if err != nil {
		return nil, err
	}
	commentDTOs, err := j.JournalCommentAssembler.ConvertToWithJournal(ctx, comments)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(commentDTOs, totalCount, commentQuery.Page), nil
}

// ListJournalCommentLatest godoc
// @Summary      查询最新日志评论
// @Description  返回指定数量的最新日志评论列表
// @Tags         Admin.Comment.Journal
// @Produce      json
// @Param        top  query     int  false  "返回数量"  example(10)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]vo.JournalCommentWithJournal}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/journals/comments/latest [get]
func (j *JournalCommentHandler) ListJournalCommentLatest(ctx *gin.Context) (interface{}, error) {
	top, err := util.MustGetQueryInt32(ctx, "top")
	if err != nil {
		return nil, err
	}
	commentQuery := param.CommentQuery{
		Sort: &param.Sort{Fields: []string{"createTime,desc"}},
		Page: param.Page{PageNum: 0, PageSize: int(top)},
	}
	comments, _, err := j.JournalCommentService.Page(ctx, commentQuery, consts.CommentTypeSheet)
	if err != nil {
		return nil, err
	}
	return j.JournalCommentAssembler.ConvertToWithJournal(ctx, comments)
}

// ListJournalCommentAsTree godoc
// @Summary      树形日志评论列表
// @Description  根据日志ID返回评论的树形结构,支持分页
// @Tags         Admin.Comment.Journal
// @Produce      json
// @Param        journalID  path     int  true  "日志ID"  example(1)
// @Param        page       query     int  false  "页码(从0开始)"  example(0)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.Comment}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/journals/comments/{journalID}/tree_view [get]
func (j *JournalCommentHandler) ListJournalCommentAsTree(ctx *gin.Context) (interface{}, error) {
	journalID, err := util.ParamInt32(ctx, "journalID")
	if err != nil {
		return nil, err
	}
	pageNum, err := util.MustGetQueryInt32(ctx, "page")
	if err != nil {
		return nil, err
	}
	pageSize, err := j.OptionService.GetOrByDefaultWithErr(ctx, property.CommentPageSize, property.CommentPageSize.DefaultValue)
	if err != nil {
		return nil, err
	}
	page := param.Page{PageSize: pageSize.(int), PageNum: int(pageNum)}

	allComments, err := j.JournalCommentService.GetByContentID(ctx, journalID, consts.CommentTypeJournal, &param.Sort{Fields: []string{"createTime,desc"}})
	if err != nil {
		return nil, err
	}

	commentVOs, totalCount, err := j.JournalCommentAssembler.PageConvertToVOs(ctx, allComments, page)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(commentVOs, totalCount, page), nil
}

// ListJournalCommentWithParent godoc
// @Summary      带父评论的日志评论列表
// @Description  根据日志ID返回评论列表(每条带父评论信息),支持分页
// @Tags         Admin.Comment.Journal
// @Produce      json
// @Param        journalID  path     int  true  "日志ID"  example(1)
// @Param        page       query     int  false  "页码(从0开始)"  example(0)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.CommentWithParent}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/journals/comments/{journalID}/list_view [get]
func (j *JournalCommentHandler) ListJournalCommentWithParent(ctx *gin.Context) (interface{}, error) {
	journalID, err := util.ParamInt32(ctx, "journalID")
	if err != nil {
		return nil, err
	}
	pageNum, err := util.MustGetQueryInt32(ctx, "page")
	if err != nil {
		return nil, err
	}

	pageSize, err := j.OptionService.GetOrByDefaultWithErr(ctx, property.CommentPageSize, property.CommentPageSize.DefaultValue)
	if err != nil {
		return nil, err
	}

	page := param.Page{PageSize: pageSize.(int), PageNum: int(pageNum)}

	comments, totalCount, err := j.JournalCommentService.Page(ctx, param.CommentQuery{
		ContentID: &journalID,
		Page:      page,
		Sort:      &param.Sort{Fields: []string{"createTime,desc"}},
	}, consts.CommentTypePost)
	if err != nil {
		return nil, err
	}

	commentsWithParent, err := j.JournalCommentAssembler.ConvertToWithParentVO(ctx, comments)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(commentsWithParent, totalCount, page), nil
}

// CreateJournalComment godoc
// @Summary      创建日志评论
// @Description  在指定日志下创建一条评论
// @Tags         Admin.Comment.Journal
// @Accept       json
// @Produce      json
// @Param        comment  body     param.AdminComment  true  "评论参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/journals/comments [post]
func (j *JournalCommentHandler) CreateJournalComment(ctx *gin.Context) (interface{}, error) {
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
	blogURL, err := j.OptionService.GetBlogBaseURL(ctx)
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
		CommentType:       consts.CommentTypeJournal,
	}
	comment, err := j.JournalCommentService.CreateBy(ctx, &commonParam)
	if err != nil {
		return nil, err
	}
	return j.JournalCommentAssembler.ConvertToDTO(ctx, comment)
}

// UpdateJournalCommentStatus godoc
// @Summary      更新日志评论状态
// @Description  根据评论ID和状态更新评论状态
// @Tags         Admin.Comment.Journal
// @Produce      json
// @Param        commentID  path     int     true  "评论ID"  example(1)
// @Param        status     path     string  true  "状态值"  example(published)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/journals/comments/{commentID}/status/{status} [put]
func (j *JournalCommentHandler) UpdateJournalCommentStatus(ctx *gin.Context) (interface{}, error) {
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
	return j.JournalCommentService.UpdateStatus(ctx, commentID, status)
}

// UpdateJournalComment godoc
// @Summary      更新日志评论
// @Description  根据评论ID更新评论内容
// @Tags         Admin.Comment.Journal
// @Accept       json
// @Produce      json
// @Param        commentID  path     int            true  "评论ID"  example(1)
// @Param        comment    body     param.Comment  true  "评论参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/journals/comments/{commentID} [put]
func (j *JournalCommentHandler) UpdateJournalComment(ctx *gin.Context) (interface{}, error) {
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
	comment, err := j.JournalCommentService.UpdateBy(ctx, commentID, commentParam)
	if err != nil {
		return nil, err
	}
	return j.JournalCommentAssembler.ConvertToDTO(ctx, comment)
}

// UpdateJournalStatusBatch godoc
// @Summary      批量更新日志评论状态
// @Description  根据评论ID列表和状态批量更新评论状态
// @Tags         Admin.Comment.Journal
// @Accept       json
// @Produce      json
// @Param        status  path     int     true  "状态值"  example(0)
// @Param        ids     body     []int   true  "评论ID列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/journals/comments/status/{status} [put]
func (j *JournalCommentHandler) UpdateJournalStatusBatch(ctx *gin.Context) (interface{}, error) {
	status, err := util.ParamInt32(ctx, "status")
	if err != nil {
		return nil, err
	}

	ids := make([]int32, 0)
	err = ctx.ShouldBindJSON(&ids)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("post ids error")
	}
	comments, err := j.JournalCommentService.UpdateStatusBatch(ctx, ids, consts.CommentStatus(status))
	if err != nil {
		return nil, err
	}
	return j.JournalCommentAssembler.ConvertToDTOList(ctx, comments)
}

// DeleteJournalComment godoc
// @Summary      删除日志评论
// @Description  根据评论ID删除指定日志评论
// @Tags         Admin.Comment.Journal
// @Produce      json
// @Param        commentID  path     int  true  "评论ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/journals/comments/{commentID} [delete]
func (j *JournalCommentHandler) DeleteJournalComment(ctx *gin.Context) (interface{}, error) {
	commentID, err := util.ParamInt32(ctx, "commentID")
	if err != nil {
		return nil, err
	}
	return nil, j.JournalCommentService.Delete(ctx, commentID)
}

// DeleteJournalCommentBatch godoc
// @Summary      批量删除日志评论
// @Description  根据评论ID列表批量删除日志评论
// @Tags         Admin.Comment.Journal
// @Accept       json
// @Produce      json
// @Param        ids  body     []int  true  "评论ID列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/journals/comments [delete]
func (j *JournalCommentHandler) DeleteJournalCommentBatch(ctx *gin.Context) (interface{}, error) {
	ids := make([]int32, 0)
	err := ctx.ShouldBindJSON(&ids)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("post ids error")
	}
	return nil, j.JournalCommentService.DeleteBatch(ctx, ids)
}
