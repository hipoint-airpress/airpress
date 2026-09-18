package admin

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/hipoint-airpress/airpress/handler/trans"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type AdminHandler struct {
	OptionService       service.OptionService
	AdminService        service.AdminService
	TwoFactorMFAService service.TwoFactorTOTPMFAService
}

func NewAdminHandler(optionService service.OptionService, adminService service.AdminService, twoFactorMFA service.TwoFactorTOTPMFAService) *AdminHandler {
	return &AdminHandler{
		OptionService:       optionService,
		AdminService:        adminService,
		TwoFactorMFAService: twoFactorMFA,
	}
}

// IsInstalled godoc
// @Summary      检查系统是否已安装
// @Description  返回是否已安装的布尔值,无需鉴权
// @Tags         Admin.Install
// @Produce      json
// @Success      200  {object}  dto.BaseDTO{data=bool}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/is_installed [get]
func (a *AdminHandler) IsInstalled(ctx *gin.Context) (interface{}, error) {
	return a.OptionService.GetOrByDefaultWithErr(ctx, property.IsInstalled, false)
}

// AuthPreCheck godoc
// @Summary      登录前预检查
// @Description  根据用户名/密码校验,返回是否需要 MFA 验证码,无需鉴权
// @Tags         Admin.Auth
// @Accept       json
// @Produce      json
// @Param        loginParam  body     param.LoginParam  true  "登录参数"
// @Success      200  {object}  dto.BaseDTO{data=dto.LoginPreCheckDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/login/precheck [post]
func (a *AdminHandler) AuthPreCheck(ctx *gin.Context) (interface{}, error) {
	var loginParam param.LoginParam
	err := ctx.ShouldBindJSON(&loginParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.BadParam.Wrapf(err, "")
	}

	user, err := a.AdminService.Authenticate(ctx, loginParam)
	if err != nil {
		return nil, err
	}
	return &dto.LoginPreCheckDTO{NeedMFACode: a.TwoFactorMFAService.UseMFA(user.MfaType)}, nil
}

// Auth godoc
// @Summary      管理员登录
// @Description  通过用户名密码(可能含 MFA 验证码)登录,返回访问令牌,无需鉴权
// @Tags         Admin.Auth
// @Accept       json
// @Produce      json
// @Param        loginParam  body     param.LoginParam  true  "登录参数"
// @Success      200  {object}  dto.BaseDTO{data=dto.AuthTokenDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/login [post]
func (a *AdminHandler) Auth(ctx *gin.Context) (interface{}, error) {
	var loginParam param.LoginParam
	err := ctx.ShouldBindJSON(&loginParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.BadParam.Wrapf(err, "").WithStatus(xerr.StatusBadRequest)
	}

	return a.AdminService.Auth(ctx, loginParam)
}

// LogOut godoc
// @Summary      退出登录
// @Description  清除当前登录用户的令牌
// @Tags         Admin.Auth
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/logout [post]
func (a *AdminHandler) LogOut(ctx *gin.Context) (interface{}, error) {
	err := a.AdminService.ClearToken(ctx)
	return nil, err
}

// SendResetCode godoc
// @Summary      发送密码重置验证码
// @Description  向管理员邮箱发送密码重置验证码
// @Tags         Admin.Auth
// @Accept       json
// @Produce      json
// @Param        resetPasswordParam  body     param.ResetPasswordParam  true  "重置密码参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/password/code [post]
func (a *AdminHandler) SendResetCode(ctx *gin.Context) (interface{}, error) {
	var resetPasswordParam param.ResetPasswordParam
	err := ctx.ShouldBindJSON(&resetPasswordParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.BadParam.Wrapf(err, "").WithStatus(xerr.StatusBadRequest)
	}
	return nil, a.AdminService.SendResetPasswordCode(ctx, resetPasswordParam)
}

// RefreshToken godoc
// @Summary      刷新访问令牌
// @Description  使用 refresh token 换取新的访问令牌,无需鉴权
// @Tags         Admin.Auth
// @Produce      json
// @Param        refreshToken  path     string  true  "刷新令牌"
// @Success      200  {object}  dto.BaseDTO{data=dto.AuthTokenDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/refresh/{refreshToken} [post]
func (a *AdminHandler) RefreshToken(ctx *gin.Context) (interface{}, error) {
	refreshToken := ctx.Param("refreshToken")
	if refreshToken == "" {
		return nil, xerr.BadParam.New("refreshToken参数为空").WithStatus(xerr.StatusBadRequest).
			WithMsg("refreshToken 参数不能为空")
	}
	return a.AdminService.RefreshToken(ctx, refreshToken)
}

// GetEnvironments godoc
// @Summary      获取环境信息
// @Description  返回运行环境信息,如数据库、启动时间、版本号、运行模式
// @Tags         Admin.Auth
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.EnvironmentDTO}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/environments [get]
func (a *AdminHandler) GetEnvironments(ctx *gin.Context) (interface{}, error) {
	return a.AdminService.GetEnvironments(ctx), nil
}

// GetLogFiles godoc
// @Summary      获取日志文件内容
// @Description  返回最近指定行数的日志文件内容
// @Tags         Admin.Auth
// @Produce      json
// @Param        lines  query     int64  false  "返回的日志行数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]string}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/airpress/logfile [get]
func (a *AdminHandler) GetLogFiles(ctx *gin.Context) (interface{}, error) {
	lines, err := util.MustGetQueryInt64(ctx, "lines")
	if err != nil {
		return nil, err
	}
	return a.AdminService.GetLogFiles(ctx, lines)
}
