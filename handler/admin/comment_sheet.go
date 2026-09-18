package admin

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/handler/binding"
	"github.com/hipoint-airpress/airpress/handler/trans"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/model/vo"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/service/assembler"
	"github.com/hipoint-airpress/airpress/service/impl"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type SheetCommentHandler struct {
	SheetCommentService   service.SheetCommentService
	BaseCommentService    service.BaseCommentService
	OptionService         service.OptionService
	SheetService          service.SheetService
	SheetAssembler        assembler.SheetAssembler
	SheetCommentAssembler assembler.SheetCommentAssembler
}

func NewSheetCommentHandler(
	sheetCommentService service.SheetCommentService,
	baseCommentService service.BaseCommentService,
	optionService service.OptionService,
	sheetService service.SheetService,
	sheetAssembler assembler.SheetAssembler,
	sheetCommentAssembler assembler.SheetCommentAssembler,
) *SheetCommentHandler {
	return &SheetCommentHandler{
		SheetCommentService:   sheetCommentService,
		BaseCommentService:    baseCommentService,
		OptionService:         optionService,
		SheetService:          sheetService,
		SheetAssembler:        sheetAssembler,
		SheetCommentAssembler: sheetCommentAssembler,
	}
}

// ListSheetComment godoc
// @Summary      分页查询页面评论列表
// @Description  支持按关键词、状态过滤,支持排序与分页
// @Tags         Admin.Comment.Sheet
// @Accept       json
// @Produce      json
// @Param        page      query     int        false  "页码(从0开始)"  example(0)
// @Param        size      query     int        false  "每页数量"        example(10)
// @Param        sort      query     []string   false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Param        keyword   query     string     false  "评论关键词"
// @Param        status    query     []string   false  "状态过滤"      collectionFormat(multi)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.SheetCommentWithSheet}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets/comments [get]
func (s *SheetCommentHandler) ListSheetComment(ctx *gin.Context) (interface{}, error) {
	var commentQuery param.CommentQuery
	err := ctx.ShouldBindWith(&commentQuery, binding.CustomFormBinding)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("Parameter error")
	}
	commentQuery.Sort = &param.Sort{
		Fields: []string{"createTime,desc"},
	}
	comments, totalCount, err := s.SheetCommentService.Page(ctx, commentQuery, consts.CommentTypeSheet)
	if err != nil {
		return nil, err
	}
	commentDTOs, err := s.ConvertToWithSheet(ctx, comments)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(commentDTOs, totalCount, commentQuery.Page), nil
}

// ListSheetCommentLatest godoc
// @Summary      查询最新页面评论
// @Description  返回指定数量的最新页面评论列表
// @Tags         Admin.Comment.Sheet
// @Produce      json
// @Param        top  query     int  false  "返回数量"  example(10)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]vo.SheetCommentWithSheet}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets/comments/latest [get]
func (s *SheetCommentHandler) ListSheetCommentLatest(ctx *gin.Context) (interface{}, error) {
	top, err := util.MustGetQueryInt32(ctx, "top")
	if err != nil {
		return nil, err
	}
	commentQuery := param.CommentQuery{
		Sort: &param.Sort{Fields: []string{"createTime,desc"}},
		Page: param.Page{PageNum: 0, PageSize: int(top)},
	}
	comments, _, err := s.SheetCommentService.Page(ctx, commentQuery, consts.CommentTypeSheet)
	if err != nil {
		return nil, err
	}
	return s.ConvertToWithSheet(ctx, comments)
}

// ListSheetCommentAsTree godoc
// @Summary      树形页面评论列表
// @Description  根据页面ID返回评论的树形结构,支持分页
// @Tags         Admin.Comment.Sheet
// @Produce      json
// @Param        sheetID  path     int  true  "页面ID"  example(1)
// @Param        page     query     int  false  "页码(从0开始)"  example(0)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.Comment}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets/comments/{sheetID}/tree_view [get]
func (s *SheetCommentHandler) ListSheetCommentAsTree(ctx *gin.Context) (interface{}, error) {
	postID, err := util.ParamInt32(ctx, "sheetID")
	if err != nil {
		return nil, err
	}
	pageNum, err := util.MustGetQueryInt32(ctx, "page")
	if err != nil {
		return nil, err
	}
	pageSize, err := s.OptionService.GetOrByDefaultWithErr(ctx, property.CommentPageSize, property.CommentPageSize.DefaultValue)
	if err != nil {
		return nil, err
	}
	page := param.Page{PageSize: pageSize.(int), PageNum: int(pageNum)}

	allComments, err := s.SheetCommentService.GetByContentID(ctx, postID, consts.CommentTypeSheet, &param.Sort{Fields: []string{"createTime,desc"}})
	if err != nil {
		return nil, err
	}
	commentVOs, totalCount, err := s.SheetCommentAssembler.PageConvertToVOs(ctx, allComments, page)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(commentVOs, totalCount, page), nil
}

// ListSheetCommentWithParent godoc
// @Summary      带父评论的页面评论列表
// @Description  根据页面ID返回评论列表(每条带父评论信息),支持分页
// @Tags         Admin.Comment.Sheet
// @Produce      json
// @Param        sheetID  path     int  true  "页面ID"  example(1)
// @Param        page     query     int  false  "页码(从0开始)"  example(0)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]vo.CommentWithParent}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets/comments/{sheetID}/list_view [get]
func (s *SheetCommentHandler) ListSheetCommentWithParent(ctx *gin.Context) (interface{}, error) {
	postID, err := util.ParamInt32(ctx, "sheetID")
	if err != nil {
		return nil, err
	}
	pageNum, err := util.MustGetQueryInt32(ctx, "page")
	if err != nil {
		return nil, err
	}

	pageSize, err := s.OptionService.GetOrByDefaultWithErr(ctx, property.CommentPageSize, property.CommentPageSize.DefaultValue)
	if err != nil {
		return nil, err
	}
	page := param.Page{PageSize: pageSize.(int), PageNum: int(pageNum)}

	comments, totalCount, err := s.SheetCommentService.Page(ctx, param.CommentQuery{
		ContentID: &postID,
		Page:      page,
		Sort:      &param.Sort{Fields: []string{"createTime,desc"}},
	}, consts.CommentTypePost)
	if err != nil {
		return nil, err
	}

	commentsWithParent, err := s.SheetCommentAssembler.ConvertToWithParentVO(ctx, comments)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(commentsWithParent, totalCount, page), nil
}

// CreateSheetComment godoc
// @Summary      创建页面评论
// @Description  在指定页面下创建一条评论
// @Tags         Admin.Comment.Sheet
// @Accept       json
// @Produce      json
// @Param        comment  body     param.AdminComment  true  "评论参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets/comments [post]
func (s *SheetCommentHandler) CreateSheetComment(ctx *gin.Context) (interface{}, error) {
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
	blogURL, err := s.OptionService.GetBlogBaseURL(ctx)
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
		CommentType:       consts.CommentTypeSheet,
	}
	comment, err := s.BaseCommentService.CreateBy(ctx, &commonParam)
	if err != nil {
		return nil, err
	}
	return s.SheetCommentAssembler.ConvertToDTO(ctx, comment)
}

// UpdateSheetCommentStatus godoc
// @Summary      更新页面评论状态
// @Description  根据评论ID和状态更新评论状态
// @Tags         Admin.Comment.Sheet
// @Produce      json
// @Param        commentID  path     int     true  "评论ID"  example(1)
// @Param        status     path     string  true  "状态值"  example(published)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets/comments/{commentID}/status/{status} [put]
func (s *SheetCommentHandler) UpdateSheetCommentStatus(ctx *gin.Context) (interface{}, error) {
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
	return s.SheetCommentService.UpdateStatus(ctx, commentID, status)
}

// UpdateSheetCommentStatusBatch godoc
// @Summary      批量更新页面评论状态
// @Description  根据评论ID列表和状态批量更新评论状态
// @Tags         Admin.Comment.Sheet
// @Accept       json
// @Produce      json
// @Param        status  path     int     true  "状态值"  example(0)
// @Param        ids     body     []int   true  "评论ID列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Comment}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets/comments/status/{status} [put]
func (s *SheetCommentHandler) UpdateSheetCommentStatusBatch(ctx *gin.Context) (interface{}, error) {
	status, err := util.ParamInt32(ctx, "status")
	if err != nil {
		return nil, err
	}

	ids := make([]int32, 0)
	err = ctx.ShouldBindJSON(&ids)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("post ids error")
	}
	comments, err := s.SheetCommentService.UpdateStatusBatch(ctx, ids, consts.CommentStatus(status))
	if err != nil {
		return nil, err
	}
	return s.SheetCommentAssembler.ConvertToDTOList(ctx, comments)
}

// DeleteSheetComment godoc
// @Summary      删除页面评论
// @Description  根据评论ID删除指定页面评论
// @Tags         Admin.Comment.Sheet
// @Produce      json
// @Param        commentID  path     int  true  "评论ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets/comments/{commentID} [delete]
func (s *SheetCommentHandler) DeleteSheetComment(ctx *gin.Context) (interface{}, error) {
	commentID, err := util.ParamInt32(ctx, "commentID")
	if err != nil {
		return nil, err
	}
	return nil, s.SheetCommentService.Delete(ctx, commentID)
}

// DeleteSheetCommentBatch godoc
// @Summary      批量删除页面评论
// @Description  根据评论ID列表批量删除页面评论
// @Tags         Admin.Comment.Sheet
// @Accept       json
// @Produce      json
// @Param        ids  body     []int  true  "评论ID列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/sheets/comments [delete]
func (s *SheetCommentHandler) DeleteSheetCommentBatch(ctx *gin.Context) (interface{}, error) {
	ids := make([]int32, 0)
	err := ctx.ShouldBindJSON(&ids)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("post ids error")
	}
	return nil, s.SheetCommentService.DeleteBatch(ctx, ids)
}

func (s *SheetCommentHandler) ConvertToWithSheet(ctx context.Context, comments []*entity.Comment) ([]*vo.SheetCommentWithSheet, error) {
	postIDs := make([]int32, 0, len(comments))
	for _, comment := range comments {
		postIDs = append(postIDs, comment.PostID)
	}
	posts, err := s.SheetService.GetByPostIDs(ctx, postIDs)
	if err != nil {
		return nil, err
	}
	result := make([]*vo.SheetCommentWithSheet, 0, len(comments))
	for _, comment := range comments {
		commentDTO, err := s.SheetCommentAssembler.ConvertToDTO(ctx, comment)
		if err != nil {
			return nil, err
		}
		commentWithSheet := &vo.SheetCommentWithSheet{
			Comment: *commentDTO,
		}
		result = append(result, commentWithSheet)
		post, ok := posts[comment.PostID]
		if ok {
			commentWithSheet.PostMinimal, err = s.SheetAssembler.ConvertToMinimalDTO(ctx, post)
			if err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}
