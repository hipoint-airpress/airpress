package api

import (
	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util"
)

type PhotoHandler struct {
	PhotoService service.PhotoService
}

func NewPhotoHandler(photoService service.PhotoService) *PhotoHandler {
	return &PhotoHandler{
		PhotoService: photoService,
	}
}

// Like godoc
// @Summary      图片点赞
// @Description  为指定图片点赞,点赞数 +1
// @Tags         Content.Photo
// @Produce      json
// @Param        photoID  path     int  true  "图片ID"  example(1)
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /content/photos/{photoID}/likes [post]
func (p *PhotoHandler) Like(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "photoID")
	if err != nil {
		return nil, err
	}
	return nil, p.PhotoService.IncreaseLike(ctx, id)
}
