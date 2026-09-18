package admin

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/hipoint-airpress/airpress/handler/trans"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type MenuHandler struct {
	MenuService service.MenuService
}

func NewMenuHandler(menuService service.MenuService) *MenuHandler {
	return &MenuHandler{
		MenuService: menuService,
	}
}

// ListMenus godoc
// @Summary      查询所有菜单
// @Description  返回所有菜单列表,支持按 team 与 priority 排序
// @Tags         Admin.Menu
// @Accept       json
// @Produce      json
// @Param        sort  query     []string  false  "排序字段,如 team,desc"  collectionFormat(multi)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Menu}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/menus [get]
func (m *MenuHandler) ListMenus(ctx *gin.Context) (interface{}, error) {
	sort := param.Sort{}
	err := ctx.ShouldBindQuery(&sort)
	if err != nil {
		return nil, xerr.WithMsg(err, "sort parameter error").WithStatus(xerr.StatusBadRequest)
	}
	if len(sort.Fields) == 0 {
		sort.Fields = append(sort.Fields, "team,desc", "priority,asc")
	} else {
		sort.Fields = append(sort.Fields, "priority,asc")
	}
	menus, err := m.MenuService.List(ctx, &sort)
	if err != nil {
		return nil, err
	}
	return m.MenuService.ConvertToDTOs(ctx, menus), nil
}

// ListMenusAsTree godoc
// @Summary      树形菜单列表
// @Description  返回所有菜单的树形结构
// @Tags         Admin.Menu
// @Accept       json
// @Produce      json
// @Param        sort  query     []string  false  "排序字段,如 team,desc"  collectionFormat(multi)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]vo.Menu}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/menus/tree_view [get]
func (m *MenuHandler) ListMenusAsTree(ctx *gin.Context) (interface{}, error) {
	sort := param.Sort{}
	err := ctx.ShouldBindQuery(&sort)
	if err != nil {
		return nil, xerr.WithMsg(err, "sort parameter error").WithStatus(xerr.StatusBadRequest)
	}
	if len(sort.Fields) == 0 {
		sort.Fields = append(sort.Fields, "team,desc", "priority,asc")
	} else {
		sort.Fields = append(sort.Fields, "priority,asc")
	}
	menus, err := m.MenuService.ListAsTree(ctx, &sort)
	if err != nil {
		return nil, err
	}
	return menus, nil
}

// ListMenusAsTreeByTeam godoc
// @Summary      按分组返回树形菜单
// @Description  根据 team 查询参数返回菜单的树形结构,team 为空时返回全部
// @Tags         Admin.Menu
// @Accept       json
// @Produce      json
// @Param        sort  query     []string  false  "排序字段,如 priority,asc"  collectionFormat(multi)
// @Param        team  query     string    false  "菜单分组名称"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]vo.Menu}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/menus/team/tree_view [get]
func (m *MenuHandler) ListMenusAsTreeByTeam(ctx *gin.Context) (interface{}, error) {
	sort := param.Sort{}
	err := ctx.ShouldBindQuery(&sort)
	if err != nil {
		return nil, xerr.WithMsg(err, "sort parameter error").WithStatus(xerr.StatusBadRequest)
	}
	if len(sort.Fields) == 0 {
		sort.Fields = append(sort.Fields, "priority,asc")
	}
	team, _ := util.MustGetQueryString(ctx, "team")
	if team == "" {
		menus, err := m.MenuService.ListAsTree(ctx, &sort)
		if err != nil {
			return nil, err
		}
		return menus, nil
	}
	menus, err := m.MenuService.ListAsTreeByTeam(ctx, team, &sort)
	if err != nil {
		return nil, err
	}
	return menus, nil
}

// GetMenuByID godoc
// @Summary      根据ID获取菜单
// @Description  返回指定 ID 的菜单详情
// @Tags         Admin.Menu
// @Produce      json
// @Param        id  path     int  true  "菜单ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Menu}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/menus/{id} [get]
func (m *MenuHandler) GetMenuByID(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	menu, err := m.MenuService.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return m.MenuService.ConvertToDTO(ctx, menu), nil
}

// CreateMenu godoc
// @Summary      创建菜单
// @Description  创建一个新菜单,返回创建后的详情
// @Tags         Admin.Menu
// @Accept       json
// @Produce      json
// @Param        menu  body     param.Menu  true  "菜单参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Menu}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/menus [post]
func (m *MenuHandler) CreateMenu(ctx *gin.Context) (interface{}, error) {
	menuParam := &param.Menu{}
	err := ctx.ShouldBindJSON(menuParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	menu, err := m.MenuService.Create(ctx, menuParam)
	if err != nil {
		return nil, err
	}
	return m.MenuService.ConvertToDTO(ctx, menu), nil
}

// CreateMenuBatch godoc
// @Summary      批量创建菜单
// @Description  根据菜单参数列表批量创建菜单
// @Tags         Admin.Menu
// @Accept       json
// @Produce      json
// @Param        menus  body     []param.Menu  true  "菜单参数列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Menu}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/menus/batch [post]
func (m *MenuHandler) CreateMenuBatch(ctx *gin.Context) (interface{}, error) {
	menuParams := make([]*param.Menu, 0)
	err := ctx.ShouldBindJSON(&menuParams)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	menus, err := m.MenuService.CreateBatch(ctx, menuParams)
	if err != nil {
		return nil, err
	}
	return m.MenuService.ConvertToDTOs(ctx, menus), nil
}

// UpdateMenu godoc
// @Summary      更新菜单
// @Description  根据菜单ID更新菜单信息
// @Tags         Admin.Menu
// @Accept       json
// @Produce      json
// @Param        id    path     int          true  "菜单ID"  example(1)
// @Param        menu  body     param.Menu   true  "菜单参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Menu}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/menus/{id} [put]
func (m *MenuHandler) UpdateMenu(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	menuParam := &param.Menu{}
	err = ctx.ShouldBindJSON(menuParam)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	menu, err := m.MenuService.Update(ctx, id, menuParam)
	if err != nil {
		return nil, err
	}
	return m.MenuService.ConvertToDTO(ctx, menu), nil
}

// UpdateMenuBatch godoc
// @Summary      批量更新菜单
// @Description  根据菜单参数列表批量更新菜单
// @Tags         Admin.Menu
// @Accept       json
// @Produce      json
// @Param        menus  body     []param.Menu  true  "菜单参数列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Menu}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/menus/batch [put]
func (m *MenuHandler) UpdateMenuBatch(ctx *gin.Context) (interface{}, error) {
	menuParams := make([]*param.Menu, 0)
	err := ctx.ShouldBindJSON(&menuParams)
	if err != nil {
		e := validator.ValidationErrors{}
		if errors.As(err, &e) {
			return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
		}
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
	}
	menus, err := m.MenuService.UpdateBatch(ctx, menuParams)
	if err != nil {
		return nil, err
	}
	return m.MenuService.ConvertToDTOs(ctx, menus), nil
}

// DeleteMenu godoc
// @Summary      删除菜单
// @Description  根据菜单ID删除指定菜单
// @Tags         Admin.Menu
// @Produce      json
// @Param        id  path     int  true  "菜单ID"  example(1)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/menus/{id} [delete]
func (m *MenuHandler) DeleteMenu(ctx *gin.Context) (interface{}, error) {
	id, err := util.ParamInt32(ctx, "id")
	if err != nil {
		return nil, err
	}
	return nil, m.MenuService.Delete(ctx, id)
}

// DeleteMenuBatch godoc
// @Summary      批量删除菜单
// @Description  根据菜单ID列表批量删除菜单
// @Tags         Admin.Menu
// @Accept       json
// @Produce      json
// @Param        ids  body     []int  true  "菜单ID列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/menus/batch [delete]
func (m *MenuHandler) DeleteMenuBatch(ctx *gin.Context) (interface{}, error) {
	menuIDs := make([]int32, 0)
	err := ctx.ShouldBind(&menuIDs)
	if err != nil {
		return nil, xerr.WithMsg(err, "menuIDs error").WithStatus(xerr.StatusBadRequest)
	}
	return nil, m.MenuService.DeleteBatch(ctx, menuIDs)
}

// ListMenuTeams godoc
// @Summary      查询菜单分组
// @Description  返回所有菜单的分组(team)列表
// @Tags         Admin.Menu
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]string}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/menus/teams [get]
func (m *MenuHandler) ListMenuTeams(ctx *gin.Context) (interface{}, error) {
	return m.MenuService.ListTeams(ctx)
}
