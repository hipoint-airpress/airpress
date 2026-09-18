package api

import (
	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util"
)

type CommentHandler struct {
	BaseCommentService service.BaseCommentService
}

func NewCommentHandler(baseCommentService service.BaseCommentService) *CommentHandler {
	return &CommentHandler{
		BaseCommentService: baseCommentService,
	}
}

// Like godoc
// @Summary      评论点赞
// @Description  为指定评论点赞,点赞数 +1
// @Tags         Content.Comment
// @Produce      json
// @Param        commentID  path     int  true  "评论ID"  example(1)
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/comments/{commentID}/likes [post]
func (c *CommentHandler) Like(ctx *gin.Context) (interface{}, error) {
	commentID, err := util.ParamInt32(ctx, "commentID")
	if err != nil {
		return nil, err
	}
	return nil, c.BaseCommentService.IncreaseLike(ctx, commentID)
}
