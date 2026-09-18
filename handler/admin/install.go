package admin

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/hipoint-airpress/airpress/handler/trans"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type InstallHandler struct {
	InstallService service.InstallService
}

func NewInstallHandler(installService service.InstallService) *InstallHandler {
	return &InstallHandler{
		InstallService: installService,
	}
}

// InstallBlog godoc
// @Summary      安装博客系统
// @Description  使用初始管理员信息等参数初始化博客系统,无需鉴权
// @Tags         Admin.Install
// @Accept       json
// @Produce      json
// @Param        install  body     param.Install  true  "安装参数"
// @Success      200  {object}  dto.BaseDTO{data=string}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/installations [post]
func (i *InstallHandler) InstallBlog(ctx *gin.Context) (interface{}, error) {
	var installParam param.Install
	err := ctx.ShouldBindJSON(&installParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest)
	}
	err = i.InstallService.InstallBlog(ctx, installParam)
	if err != nil {
		return nil, err
	}
	return "安装完成", nil
}
