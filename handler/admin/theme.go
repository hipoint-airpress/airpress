package admin

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/handler/trans"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type ThemeHandler struct {
	ThemeService  service.ThemeService
	OptionService service.OptionService
}

func NewThemeHandler(l service.ThemeService, o service.OptionService) *ThemeHandler {
	return &ThemeHandler{
		ThemeService:  l,
		OptionService: o,
	}
}

// GetThemeByID godoc
// @Summary      根据ID获取主题
// @Description  返回指定 ID 的主题详情
// @Tags         Admin.Theme
// @Produce      json
// @Param        themeID  path     string  true  "主题ID"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.ThemeProperty}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/{themeID} [get]
func (t *ThemeHandler) GetThemeByID(ctx *gin.Context) (interface{}, error) {
	themeID, err := util.ParamString(ctx, "themeID")
	if err != nil {
		return nil, err
	}
	return t.ThemeService.GetThemeByID(ctx, themeID)
}

// ListAllThemes godoc
// @Summary      查询所有主题
// @Description  返回所有可用主题列表
// @Tags         Admin.Theme
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.ThemeProperty}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes [get]
func (t *ThemeHandler) ListAllThemes(ctx *gin.Context) (interface{}, error) {
	return t.ThemeService.ListAllTheme(ctx)
}

// ListActivatedThemeFile godoc
// @Summary      列出当前激活主题的文件
// @Description  返回当前激活主题下所有模板/静态文件列表
// @Tags         Admin.Theme
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/activation/files [get]
func (t *ThemeHandler) ListActivatedThemeFile(ctx *gin.Context) (interface{}, error) {
	activatedThemeID, err := t.OptionService.GetActivatedThemeID(ctx)
	if err != nil {
		return nil, err
	}
	return t.ThemeService.ListThemeFiles(ctx, activatedThemeID)
}

// ListThemeFileByID godoc
// @Summary      按主题ID列出主题文件
// @Description  返回指定主题ID下的所有模板/静态文件列表
// @Tags         Admin.Theme
// @Produce      json
// @Param        themeID  path  string  true  "主题ID"  example(default-theme)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/{themeID}/files [get]
func (t *ThemeHandler) ListThemeFileByID(ctx *gin.Context) (interface{}, error) {
	themeID, err := util.ParamString(ctx, "themeID")
	if err != nil {
		return nil, err
	}
	return t.ThemeService.ListThemeFiles(ctx, themeID)
}

// GetThemeFileContent godoc
// @Summary      获取当前激活主题指定文件内容
// @Description  按文件相对路径返回当前激活主题中该文件的文本内容
// @Tags         Admin.Theme
// @Produce      json
// @Param        path  query  string  true  "文件相对路径"  example(header.ftl)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/files/content [get]
func (t *ThemeHandler) GetThemeFileContent(ctx *gin.Context) (interface{}, error) {
	path, err := util.MustGetQueryString(ctx, "path")
	if err != nil {
		return nil, err
	}
	activatedThemeID, err := t.OptionService.GetActivatedThemeID(ctx)
	if err != nil {
		return nil, err
	}
	return t.ThemeService.GetThemeFileContent(ctx, activatedThemeID, path)
}

// GetThemeFileContentByID godoc
// @Summary      按主题ID获取指定文件内容
// @Description  按主题ID和文件相对路径返回该文件的文本内容
// @Tags         Admin.Theme
// @Produce      json
// @Param        themeID  path  string  true  "主题ID"  example(default-theme)
// @Param        path     query string  true  "文件相对路径"  example(header.ftl)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/{themeID}/files/content [get]
func (t *ThemeHandler) GetThemeFileContentByID(ctx *gin.Context) (interface{}, error) {
	themeID, err := util.ParamString(ctx, "themeID")
	if err != nil {
		return nil, err
	}
	path, err := util.MustGetQueryString(ctx, "path")
	if err != nil {
		return nil, err
	}

	return t.ThemeService.GetThemeFileContent(ctx, themeID, path)
}

// UpdateThemeFile godoc
// @Summary      更新当前激活主题指定文件内容
// @Description  按文件路径更新当前激活主题中该文件的文本内容
// @Tags         Admin.Theme
// @Accept       json
// @Produce      json
// @Param        body  body  param.ThemeContent  true  "主题文件内容参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/files/content [put]
func (t *ThemeHandler) UpdateThemeFile(ctx *gin.Context) (interface{}, error) {
	themeParam := &param.ThemeContent{}
	err := ctx.ShouldBindJSON(themeParam)
	if err != nil {
		if err != nil {
			e := validator.ValidationErrors{}
			if errors.As(err, &e) {
				return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
			}
			return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
		}
	}
	activatedThemeID, err := t.OptionService.GetActivatedThemeID(ctx)
	if err != nil {
		return nil, err
	}
	return nil, t.ThemeService.UpdateThemeFile(ctx, activatedThemeID, themeParam.Path, themeParam.Content)
}

// UpdateThemeFileByID godoc
// @Summary      按主题ID更新指定文件内容
// @Description  按主题ID和文件路径更新该主题中指定文件的文本内容
// @Tags         Admin.Theme
// @Accept       json
// @Produce      json
// @Param        themeID  path  string             true  "主题ID"  example(default-theme)
// @Param        body     body  param.ThemeContent true  "主题文件内容参数"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/{themeID}/files/content [put]
func (t *ThemeHandler) UpdateThemeFileByID(ctx *gin.Context) (interface{}, error) {
	themeID, err := util.ParamString(ctx, "themeID")
	if err != nil {
		return nil, err
	}
	themeParam := &param.ThemeContent{}
	err = ctx.ShouldBindJSON(themeParam)
	if err != nil {
		if err != nil {
			e := validator.ValidationErrors{}
			if errors.As(err, &e) {
				return nil, xerr.WithStatus(e, xerr.StatusBadRequest).WithMsg(trans.Translate(e))
			}
			return nil, xerr.WithStatus(err, xerr.StatusBadRequest).WithMsg("parameter error")
		}
	}
	return nil, t.ThemeService.UpdateThemeFile(ctx, themeID, themeParam.Path, themeParam.Content)
}

// ListCustomSheetTemplate godoc
// @Summary      列出当前激活主题的自定义页面模板
// @Description  返回当前激活主题下所有自定义 sheet 模板列表
// @Tags         Admin.Theme
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/activation/template/custom/sheet [get]
func (t *ThemeHandler) ListCustomSheetTemplate(ctx *gin.Context) (interface{}, error) {
	activatedThemeID, err := t.OptionService.GetActivatedThemeID(ctx)
	if err != nil {
		return nil, err
	}
	return t.ThemeService.ListCustomTemplates(ctx, activatedThemeID, consts.ThemeCustomSheetPrefix)
}

// ListCustomPostTemplate godoc
// @Summary      列出当前激活主题的自定义文章模板
// @Description  返回当前激活主题下所有自定义 post 模板列表
// @Tags         Admin.Theme
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/activation/template/custom/post [get]
func (t *ThemeHandler) ListCustomPostTemplate(ctx *gin.Context) (interface{}, error) {
	activatedThemeID, err := t.OptionService.GetActivatedThemeID(ctx)
	if err != nil {
		return nil, err
	}
	return t.ThemeService.ListCustomTemplates(ctx, activatedThemeID, consts.ThemeCustomPostPrefix)
}

// ActivateTheme godoc
// @Summary      激活指定主题
// @Description  将指定主题ID的主题设为当前激活主题
// @Tags         Admin.Theme
// @Produce      json
// @Param        themeID  path  string  true  "主题ID"  example(default-theme)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/{themeID}/activation [post]
func (t *ThemeHandler) ActivateTheme(ctx *gin.Context) (interface{}, error) {
	themeID, err := util.ParamString(ctx, "themeID")
	if err != nil {
		return nil, err
	}
	return t.ThemeService.ActivateTheme(ctx, themeID)
}

// GetActivatedTheme godoc
// @Summary      获取当前激活主题
// @Description  返回当前激活主题的详情
// @Tags         Admin.Theme
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/activation [get]
func (t *ThemeHandler) GetActivatedTheme(ctx *gin.Context) (interface{}, error) {
	activatedThemeID, err := t.OptionService.GetActivatedThemeID(ctx)
	if err != nil {
		return nil, err
	}
	return t.ThemeService.GetThemeByID(ctx, activatedThemeID)
}

// GetActivatedThemeConfig godoc
// @Summary      获取当前激活主题的配置
// @Description  返回当前激活主题的配置项分组列表
// @Tags         Admin.Theme
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/activation/configurations [get]
func (t *ThemeHandler) GetActivatedThemeConfig(ctx *gin.Context) (interface{}, error) {
	activatedThemeID, err := t.OptionService.GetActivatedThemeID(ctx)
	if err != nil {
		return nil, err
	}
	return t.ThemeService.GetThemeConfig(ctx, activatedThemeID)
}

// GetThemeConfigByID godoc
// @Summary      按主题ID获取主题配置
// @Description  返回指定主题ID的配置项分组列表
// @Tags         Admin.Theme
// @Produce      json
// @Param        themeID  path  string  true  "主题ID"  example(default-theme)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/{themeID}/configurations [get]
func (t *ThemeHandler) GetThemeConfigByID(ctx *gin.Context) (interface{}, error) {
	themeID, err := util.ParamString(ctx, "themeID")
	if err != nil {
		return nil, err
	}
	return t.ThemeService.GetThemeConfig(ctx, themeID)
}

// GetThemeConfigByGroup godoc
// @Summary      按分组获取主题配置项
// @Description  返回指定主题ID下指定分组的配置项列表
// @Tags         Admin.Theme
// @Produce      json
// @Param        themeID  path  string  true  "主题ID"  example(default-theme)
// @Param        group    path  string  true  "分组名称"  example(basic)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/{themeID}/configurations/groups/{group} [get]
func (t *ThemeHandler) GetThemeConfigByGroup(ctx *gin.Context) (interface{}, error) {
	themeID, err := util.ParamString(ctx, "themeID")
	if err != nil {
		return nil, err
	}
	group, err := util.ParamString(ctx, "group")
	if err != nil {
		return nil, err
	}
	themeSettings, err := t.ThemeService.GetThemeConfig(ctx, themeID)
	if err != nil {
		return nil, err
	}
	for _, setting := range themeSettings {
		if setting.Name == group {
			return setting.Items, nil
		}
	}
	return nil, nil
}

// GetThemeConfigGroupNames godoc
// @Summary      获取主题配置分组名列表
// @Description  返回指定主题ID下所有配置项分组名称
// @Tags         Admin.Theme
// @Produce      json
// @Param        themeID  path  string  true  "主题ID"  example(default-theme)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]string}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/{themeID}/configurations/groups [get]
func (t *ThemeHandler) GetThemeConfigGroupNames(ctx *gin.Context) (interface{}, error) {
	themeID, err := util.ParamString(ctx, "themeID")
	if err != nil {
		return nil, err
	}
	themeSettings, err := t.ThemeService.GetThemeConfig(ctx, themeID)
	if err != nil {
		return nil, err
	}
	groupNames := make([]string, len(themeSettings))
	for index, setting := range themeSettings {
		groupNames[index] = setting.Name
	}
	return groupNames, nil
}

// GetActivatedThemeSettingMap godoc
// @Summary      获取当前激活主题的设置项map
// @Description  返回当前激活主题的所有设置项,以 key-value map 形式
// @Tags         Admin.Theme
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/activation/settings [get]
func (t *ThemeHandler) GetActivatedThemeSettingMap(ctx *gin.Context) (interface{}, error) {
	activatedThemeID, err := t.OptionService.GetActivatedThemeID(ctx)
	if err != nil {
		return nil, err
	}
	return t.ThemeService.GetThemeSettingMap(ctx, activatedThemeID)
}

// GetThemeSettingMapByID godoc
// @Summary      按主题ID获取主题设置项map
// @Description  返回指定主题ID的所有设置项,以 key-value map 形式
// @Tags         Admin.Theme
// @Produce      json
// @Param        themeID  path  string  true  "主题ID"  example(default-theme)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/{themeID}/settings [get]
func (t *ThemeHandler) GetThemeSettingMapByID(ctx *gin.Context) (interface{}, error) {
	themeID, err := util.ParamString(ctx, "themeID")
	if err != nil {
		return nil, err
	}
	return t.ThemeService.GetThemeSettingMap(ctx, themeID)
}

// GetThemeSettingMapByGroupAndThemeID godoc
// @Summary      按主题ID和分组获取设置项map
// @Description  返回指定主题ID下指定分组的所有设置项,以 key-value map 形式
// @Tags         Admin.Theme
// @Produce      json
// @Param        themeID  path  string  true  "主题ID"  example(default-theme)
// @Param        group    path  string  true  "分组名称"  example(basic)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/{themeID}/groups/{group}/settings [get]
func (t *ThemeHandler) GetThemeSettingMapByGroupAndThemeID(ctx *gin.Context) (interface{}, error) {
	themeID, err := util.ParamString(ctx, "themeID")
	if err != nil {
		return nil, err
	}
	group, err := util.ParamString(ctx, "group")
	if err != nil {
		return nil, err
	}
	return t.ThemeService.GetThemeGroupSettingMap(ctx, themeID, group)
}

// SaveActivatedThemeSetting godoc
// @Summary      保存当前激活主题的设置项
// @Description  以 map 形式批量保存当前激活主题的设置项
// @Tags         Admin.Theme
// @Accept       json
// @Produce      json
// @Param        settings  body  map[string]interface{}  true  "设置项key-value"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/activation/settings [post]
func (t *ThemeHandler) SaveActivatedThemeSetting(ctx *gin.Context) (interface{}, error) {
	activatedThemeID, err := t.OptionService.GetActivatedThemeID(ctx)
	if err != nil {
		return nil, err
	}
	settings := make(map[string]interface{})
	err = ctx.ShouldBindJSON(&settings)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest)
	}
	return nil, t.ThemeService.SaveThemeSettings(ctx, activatedThemeID, settings)
}

// SaveThemeSettingByID godoc
// @Summary      按主题ID保存主题设置项
// @Description  以 map 形式批量保存指定主题ID的设置项
// @Tags         Admin.Theme
// @Accept       json
// @Produce      json
// @Param        themeID   path  string                  true  "主题ID"  example(default-theme)
// @Param        settings  body  map[string]interface{}  true  "设置项key-value"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/{themeID}/settings [post]
func (t *ThemeHandler) SaveThemeSettingByID(ctx *gin.Context) (interface{}, error) {
	themeID, err := util.ParamString(ctx, "themeID")
	if err != nil {
		return nil, err
	}
	settings := make(map[string]interface{})
	err = ctx.ShouldBindJSON(&settings)
	if err != nil {
		return nil, xerr.WithStatus(err, xerr.StatusBadRequest)
	}
	return nil, t.ThemeService.SaveThemeSettings(ctx, themeID, settings)
}

// DeleteThemeByID godoc
// @Summary      按主题ID删除主题
// @Description  删除指定主题ID,可选是否同时删除其设置项
// @Tags         Admin.Theme
// @Produce      json
// @Param        themeID         path  string  true  "主题ID"  example(default-theme)
// @Param        deleteSettings  query bool    false "是否同时删除主题设置项"  example(false)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/{themeID} [delete]
func (t *ThemeHandler) DeleteThemeByID(ctx *gin.Context) (interface{}, error) {
	themeID, err := util.ParamString(ctx, "themeID")
	if err != nil {
		return nil, err
	}
	isDeleteSetting, err := util.GetQueryBool(ctx, "deleteSettings", false)
	if err != nil {
		return nil, err
	}
	return nil, t.ThemeService.DeleteTheme(ctx, themeID, isDeleteSetting)
}

// UploadTheme godoc
// @Summary      上传主题
// @Description  上传主题压缩文件,新增一个主题
// @Tags         Admin.Theme
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "主题压缩包"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/upload [post]
func (t *ThemeHandler) UploadTheme(ctx *gin.Context) (interface{}, error) {
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		return nil, xerr.WithMsg(err, "upload theme error").WithStatus(xerr.StatusBadRequest)
	}
	return t.ThemeService.UploadTheme(ctx, fileHeader)
}

// UpdateThemeByUpload godoc
// @Summary      按主题ID上传覆盖主题
// @Description  上传主题压缩文件,覆盖指定主题ID的主题
// @Tags         Admin.Theme
// @Accept       multipart/form-data
// @Produce      json
// @Param        themeID  path  string  true  "主题ID"  example(default-theme)
// @Param        file     formData  file  true  "主题压缩包"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/upload/{themeID} [put]
func (t *ThemeHandler) UpdateThemeByUpload(ctx *gin.Context) (interface{}, error) {
	themeID, err := util.ParamString(ctx, "themeID")
	if err != nil {
		return nil, err
	}
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		return nil, xerr.WithMsg(err, "upload theme error").WithStatus(xerr.StatusBadRequest)
	}
	return t.ThemeService.UpdateThemeByUpload(ctx, themeID, fileHeader)
}

// FetchTheme godoc
// @Summary      从远程URL拉取主题
// @Description  通过指定 URI 下载并安装主题
// @Tags         Admin.Theme
// @Produce      json
// @Param        uri  query  string  false  "主题远程下载地址"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/fetching [post]
func (t *ThemeHandler) FetchTheme(ctx *gin.Context) (interface{}, error) {
	uri, _ := util.MustGetQueryString(ctx, "uri")
	return t.ThemeService.Fetch(ctx, uri)
}

// UpdateThemeByFetching godoc
// @Summary      按主题ID从远程URL拉取并覆盖主题
// @Description  目前未实现,固定返回 500 not support
// @Tags         Admin.Theme
// @Produce      json
// @Param        themeID  path  string  true  "主题ID"  example(default-theme)
// @Security     AdminApiKey
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/fetching/{themeID} [put]
func (t *ThemeHandler) UpdateThemeByFetching(ctx *gin.Context) (interface{}, error) {
	return nil, xerr.WithMsg(nil, "not support").WithStatus(xerr.StatusInternalServerError)
}

// ReloadTheme godoc
// @Summary      重新加载主题
// @Description  重新扫描并加载所有主题文件,刷新内存中的主题缓存
// @Tags         Admin.Theme
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/reload [post]
func (t *ThemeHandler) ReloadTheme(ctx *gin.Context) (interface{}, error) {
	return nil, t.ThemeService.ReloadTheme(ctx)
}

// TemplateExist godoc
// @Summary      检查模板是否存在
// @Description  检查当前激活主题中指定模板是否存在
// @Tags         Admin.Theme
// @Produce      json
// @Param        template  query  string  true  "模板名称"  example(post)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=bool}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/themes/activation/template/exists [get]
func (t *ThemeHandler) TemplateExist(ctx *gin.Context) (interface{}, error) {
	template, err := util.MustGetQueryString(ctx, "template")
	if err != nil {
		return nil, err
	}
	return t.ThemeService.TemplateExist(ctx, template)
}
