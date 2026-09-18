package api

import (
	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
)

type OptionHandler struct {
	OptionService service.OptionService
}

func NewOptionHandler(
	optionService service.OptionService,
) *OptionHandler {
	return &OptionHandler{
		OptionService: optionService,
	}
}

// Comment godoc
// @Summary      查询评论相关配置
// @Description  返回前台评论模块使用的配置项(Gravatar 源、默认头像、内容占位符等)
// @Tags         Content.Option
// @Produce      json
// @Success      200  {object}  dto.BaseDTO{data=map[string]interface{}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/options/comment [get]
func (o *OptionHandler) Comment(ctx *gin.Context) (interface{}, error) {
	result := make(map[string]interface{})

	result[property.CommentGravatarSource.KeyValue] = o.OptionService.GetOrByDefault(ctx, property.CommentGravatarSource)
	result[property.CommentGravatarDefault.KeyValue] = o.OptionService.GetOrByDefault(ctx, property.CommentGravatarDefault)
	result[property.CommentContentPlaceholder.KeyValue] = o.OptionService.GetOrByDefault(ctx, property.CommentContentPlaceholder)
	return result, nil
}
