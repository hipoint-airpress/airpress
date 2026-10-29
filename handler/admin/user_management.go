package admin

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/handler/trans"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type UserManagementHandler struct {
	UserManagementService service.UserManagementService
}

func NewUserManagementHandler(userManagementService service.UserManagementService) *UserManagementHandler {
	return &UserManagementHandler{UserManagementService: userManagementService}
}

func (h *UserManagementHandler) ListUsers(ctx *gin.Context) (interface{}, error) {
	query := param.UserQuery{}
	if err := ctx.ShouldBindQuery(&query); err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	users, total, err := h.UserManagementService.List(ctx, query)
	if err != nil {
		return nil, err
	}
	return dto.NewPage(users, total, query.Page), nil
}

func (h *UserManagementHandler) GetUser(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	return h.UserManagementService.Get(ctx, id)
}

func (h *UserManagementHandler) CreateUser(ctx *gin.Context) (interface{}, error) {
	userParam := &param.CreateUser{}
	if err := ctx.ShouldBindJSON(userParam); err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	return h.UserManagementService.Create(ctx, *userParam)
}

func (h *UserManagementHandler) UpdateUser(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	userParam := &param.UpdateUser{}
	if err := ctx.ShouldBindJSON(userParam); err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	return h.UserManagementService.Update(ctx, id, *userParam)
}

func (h *UserManagementHandler) UpdateUserRole(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	roleParam := &param.UpdateUserRole{}
	if err := ctx.ShouldBindJSON(roleParam); err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	return nil, h.UserManagementService.UpdateRole(ctx, id, roleParam.Role)
}

func (h *UserManagementHandler) UpdateUserStatus(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	statusParam := &param.UpdateUserStatus{}
	if err := ctx.ShouldBindJSON(statusParam); err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	return nil, h.UserManagementService.UpdateStatus(ctx, id, statusParam.Disabled)
}

func (h *UserManagementHandler) ResetUserPassword(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	passwordParam := &param.ResetUserPassword{}
	if err := ctx.ShouldBindJSON(passwordParam); err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	return nil, h.UserManagementService.ResetPassword(ctx, id, passwordParam.NewPassword)
}

func (h *UserManagementHandler) GetUserContentCount(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	return h.UserManagementService.ContentCount(ctx, id)
}

func (h *UserManagementHandler) DeleteUser(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	transferToID := int32(0)
	if value := ctx.Query("transferToUserId"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("transferToUserId parameter error")
		}
		transferToID = int32(parsed)
	}
	return h.UserManagementService.Delete(ctx, id, transferToID)
}

func (h *UserManagementHandler) GetMFAPolicy(ctx *gin.Context) (interface{}, error) {
	policy, err := h.UserManagementService.GetMFAPolicy(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"policy": policy}, nil
}

func (h *UserManagementHandler) SaveMFAPolicy(ctx *gin.Context) (interface{}, error) {
	type Param struct {
		Policy consts.MFAPolicy `json:"policy" binding:"required"`
	}
	policyParam := &Param{}
	if err := ctx.ShouldBindJSON(policyParam); err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	return nil, h.UserManagementService.SaveMFAPolicy(ctx, policyParam.Policy)
}

// ListSelectableUsers godoc
// @Summary      查询可选择的用户
// @Description  返回未禁用用户的精简信息，供编辑角色选择文章目标作者
// @Tags         Admin.UserManagement
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.SelectableUser}
// @Failure      403  {object}  dto.BaseDTO
// @Router       /admin/users/selectable [get]
func (h *UserManagementHandler) ListSelectableUsers(ctx *gin.Context) (interface{}, error) {
	return h.UserManagementService.ListSelectable(ctx)
}
