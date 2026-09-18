package api

import (
	"html/template"

	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/handler/binding"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/service/assembler"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type JournalHandler struct {
	JournalService          service.JournalService
	JournalCommentService   service.JournalCommentService
	OptionService           service.ClientOptionService
	JournalCommentAssembler assembler.JournalCommentAssembler
}

func NewJournalHandler(
	journalService service.JournalService,
	journalCommentService service.JournalCommentService,
	optionService service.ClientOptionService,
	journalCommentAssembler assembler.JournalCommentAssembler,
) *JournalHandler {
	return &JournalHandler{
		JournalService:          journalService,
		JournalCommentService:   journalCommentService,
		OptionService:           optionService,
		JournalCommentAssembler: journalCommentAssembler,
	}
}

// ListJournal godoc
// @Summary      分页查询日志列表
// @Description  返回公开类型的日志,按创建时间倒序分页
// @Tags         Content.Journal
// @Produce      json
// @Param        page     query     int       false  "页码(从0开始)"          example(0)
// @Param        size     query     int       false  "每页数量"              example(10)
// @Param        sort     query     []string  false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Param        keyword  query     string    false  "日志关键词"
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]dto.JournalWithComment}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/journals [get]
func (j *JournalHandler) ListJournal(ctx *gin.Context) (interface{}, error) {
	var journalQuery param.JournalQuery
	err := ctx.ShouldBindWith(&journalQuery, binding.CustomFormBinding)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	journalQuery.Sort = &param.Sort{
		Fields: []string{"createTime,desc"},
	}
	journalQuery.JournalType = consts.JournalTypePublic.Ptr()
	journals, totalCount, err := j.JournalService.ListJournal(ctx, journalQuery)
	if err != nil {
		return nil, err
	}
	journalDTOs, err := j.JournalService.ConvertToWithCommentDTOList(ctx, journals)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(journalDTOs, totalCount, journalQuery.Page), nil
}

// GetJournal godoc
// @Summary      根据日志ID获取详情
// @Description  返回指定日志的详情(带评论数)
// @Tags         Content.Journal
// @Produce      json
// @Param        journalID  path     int  true  "日志ID"  example(1)
// @Success      200  {object}  dto.BaseDTO{data=dto.JournalWithComment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/journals/{journalID} [get]
func (j *JournalHandler) GetJournal(ctx *gin.Context) (interface{}, error) {
	journalID, err := util.ParamInt32(ctx, "journalID")
	if err != nil {
		return nil, err
	}
	journals, err := j.JournalService.GetByJournalIDs(ctx, []int32{journalID})
	if err != nil {
		return nil, err
	}
	if len(journals) == 0 {
		return nil, xerr.WithStatus(nil, xerr.StatusBadRequest)
	}
	journalDTOs, err := j.JournalService.ConvertToWithCommentDTOList(ctx, []*entity.Journal{journals[journalID]})
	if err != nil {
		return nil, err
	}
	return journalDTOs[0], nil
}

// ListTopComment godoc
// @Summary      查询日志的顶级评论
// @Description  分页返回指定日志下已发布的顶级评论(带是否有子评论标识)
// @Tags         Content.Journal
// @Produce      json
// @Param        journalID  path     int       true  "日志ID"  example(1)
// @Param        page       query     int       false  "页码(从0开始)"          example(0)
// @Param        size       query     int       false  "每页数量"              example(10)
// @Param        sort       query     []string  false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.CommentWithHasChildren}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/journals/{journalID}/comments/top_view [get]
func (j *JournalHandler) ListTopComment(ctx *gin.Context) (interface{}, error) {
	journalID, err := util.ParamInt32(ctx, "journalID")
	if err != nil {
		return nil, err
	}
	pageSize := j.OptionService.GetOrByDefault(ctx, property.CommentPageSize).(int)

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
	commentQuery.ContentID = &journalID
	commentQuery.Keyword = nil
	commentQuery.CommentStatus = consts.CommentStatusPublished.Ptr()
	commentQuery.PageSize = pageSize
	commentQuery.ParentID = util.Int32Ptr(0)

	comments, totalCount, err := j.JournalCommentService.Page(ctx, commentQuery, consts.CommentTypeJournal)
	if err != nil {
		return nil, err
	}
	_ = j.JournalCommentAssembler.ClearSensitiveField(ctx, comments)
	commenVOs, err := j.JournalCommentAssembler.ConvertToWithHasChildren(ctx, comments)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(commenVOs, totalCount, commentQuery.Page), nil
}

// ListChildren godoc
// @Summary      查询日志评论的子评论
// @Description  返回指定日志下某条评论的全部子评论
// @Tags         Content.Journal
// @Produce      json
// @Param        journalID  path     int  true  "日志ID"     example(1)
// @Param        parentID   path     int  true  "父评论ID"  example(1)
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/journals/{journalID}/comments/{parentID}/children [get]
func (j *JournalHandler) ListChildren(ctx *gin.Context) (interface{}, error) {
	journalID, err := util.ParamInt32(ctx, "journalID")
	if err != nil {
		return nil, err
	}
	parentID, err := util.ParamInt32(ctx, "parentID")
	if err != nil {
		return nil, err
	}
	children, err := j.JournalCommentService.GetChildren(ctx, parentID, journalID, consts.CommentTypeJournal)
	if err != nil {
		return nil, err
	}
	_ = j.JournalCommentAssembler.ClearSensitiveField(ctx, children)
	return j.JournalCommentAssembler.ConvertToDTOList(ctx, children)
}

// ListCommentTree godoc
// @Summary      查询日志评论的树形视图
// @Description  分页返回指定日志下全部已发布评论并按父子关系组织为树形结构
// @Tags         Content.Journal
// @Produce      json
// @Param        journalID  path     int       true  "日志ID"  example(1)
// @Param        page       query     int       false  "页码(从0开始)"          example(0)
// @Param        size       query     int       false  "每页数量"              example(10)
// @Param        sort       query     []string  false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.Comment}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/journals/{journalID}/comments/tree_view [get]
func (j *JournalHandler) ListCommentTree(ctx *gin.Context) (interface{}, error) {
	journalID, err := util.ParamInt32(ctx, "journalID")
	if err != nil {
		return nil, err
	}
	pageSize := j.OptionService.GetOrByDefault(ctx, property.CommentPageSize).(int)

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
	commentQuery.ContentID = &journalID
	commentQuery.Keyword = nil
	commentQuery.CommentStatus = consts.CommentStatusPublished.Ptr()
	commentQuery.PageSize = pageSize
	commentQuery.ParentID = util.Int32Ptr(0)

	allComments, err := j.JournalCommentService.GetByContentID(ctx, journalID, consts.CommentTypeJournal, commentQuery.Sort)
	if err != nil {
		return nil, err
	}
	_ = j.JournalCommentAssembler.ClearSensitiveField(ctx, allComments)
	commentVOs, total, err := j.JournalCommentAssembler.PageConvertToVOs(ctx, allComments, commentQuery.Page)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(commentVOs, total, commentQuery.Page), nil
}

// ListComment godoc
// @Summary      查询日志评论的列表视图
// @Description  分页返回指定日志下已发布评论,每条评论携带其父评论信息
// @Tags         Content.Journal
// @Produce      json
// @Param        journalID  path     int       true  "日志ID"  example(1)
// @Param        page       query     int       false  "页码(从0开始)"          example(0)
// @Param        size       query     int       false  "每页数量"              example(10)
// @Param        sort       query     []string  false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.CommentWithParent}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/journals/{journalID}/comments/list_view [get]
func (j *JournalHandler) ListComment(ctx *gin.Context) (interface{}, error) {
	journalID, err := util.ParamInt32(ctx, "journalID")
	if err != nil {
		return nil, err
	}
	pageSize := j.OptionService.GetOrByDefault(ctx, property.CommentPageSize).(int)

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
	commentQuery.ContentID = &journalID
	commentQuery.Keyword = nil
	commentQuery.CommentStatus = consts.CommentStatusPublished.Ptr()
	commentQuery.PageSize = pageSize
	commentQuery.ParentID = util.Int32Ptr(0)

	comments, total, err := j.JournalCommentService.Page(ctx, commentQuery, consts.CommentTypeJournal)
	if err != nil {
		return nil, err
	}
	_ = j.JournalCommentAssembler.ClearSensitiveField(ctx, comments)
	result, err := j.JournalCommentAssembler.ConvertToWithParentVO(ctx, comments)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(result, total, commentQuery.Page), nil
}

// CreateComment godoc
// @Summary      创建日志评论
// @Description  为指定日志创建一条新评论,作者、邮箱、内容会做 HTML 转义
// @Tags         Content.Journal
// @Accept       json
// @Produce      json
// @Param        comment  body     param.Comment  true  "评论参数"
// @Success      200  {object}  dto.BaseDTO{data=dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/journals/comments [post]
func (j *JournalHandler) CreateComment(ctx *gin.Context) (interface{}, error) {
	p := param.Comment{}
	err := ctx.ShouldBindJSON(&p)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	if p.AuthorURL != "" {
		err = util.Validate.Var(p.AuthorURL, "http_url")
		if err != nil {
			return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
		}
	}
	p.Author = template.HTMLEscapeString(p.Author)
	p.AuthorURL = template.HTMLEscapeString(p.AuthorURL)
	p.Content = template.HTMLEscapeString(p.Content)
	p.Email = template.HTMLEscapeString(p.Email)
	p.CommentType = consts.CommentTypeJournal
	result, err := j.JournalCommentService.CreateBy(ctx, &p)
	if err != nil {
		return nil, err
	}
	return j.JournalCommentAssembler.ConvertToDTO(ctx, result)
}

// Like godoc
// @Summary      日志点赞
// @Description  为指定日志点赞,点赞数 +1
// @Tags         Content.Journal
// @Produce      json
// @Param        journalID  path     int  true  "日志ID"  example(1)
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/journals/{journalID}/likes [post]
func (j *JournalHandler) Like(ctx *gin.Context) (interface{}, error) {
	journalID, err := util.ParamInt32(ctx, "journalID")
	if err != nil {
		return nil, err
	}
	err = j.JournalService.IncreaseLike(ctx, journalID)
	if err != nil {
		return nil, err
	}
	return nil, err
}
