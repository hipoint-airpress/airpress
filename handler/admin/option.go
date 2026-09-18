package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type OptionHandler struct {
	OptionService service.OptionService
}

func NewOptionHandler(optionService service.OptionService) *OptionHandler {
	return &OptionHandler{
		OptionService: optionService,
	}
}

// ListAllOptions godoc
// @Summary      查询所有选项
// @Description  返回系统所有配置选项列表
// @Tags         Admin.Option
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Option}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/options [get]
func (o *OptionHandler) ListAllOptions(ctx *gin.Context) (interface{}, error) {
	return o.OptionService.ListAllOption(ctx)
}

// SaveOption godoc
// @Summary      保存选项
// @Description  批量保存配置选项
// @Tags         Admin.Option
// @Accept       json
// @Produce      json
// @Param        options  body     []param.Option  true  "选项列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/options/saving [post]
func (o *OptionHandler) SaveOption(ctx *gin.Context) (interface{}, error) {
	optionParams := make([]*param.Option, 0)
	err := ctx.ShouldBindJSON(&optionParams)
	if err != nil {
		return nil, xerr.WithMsg(err, "param error").WithStatus(xerr.StatusBadRequest)
	}
	optionMap := make(map[string]string, 0)
	for _, option := range optionParams {
		optionMap[option.Key] = option.Value
	}
	return nil, o.OptionService.Save(ctx, optionMap)
}

// ListAllOptionsAsMap godoc
// @Summary      以 Map 形式查询所有选项
// @Description  返回 key-value 形式的所有配置选项
// @Tags         Admin.Option
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=map[string]interface{}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/options/map_view [get]
func (o *OptionHandler) ListAllOptionsAsMap(ctx *gin.Context) (interface{}, error) {
	options, err := o.OptionService.ListAllOption(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]interface{})
	for _, option := range options {
		result[option.Key] = option.Value
	}
	return result, nil
}

// ListAllOptionsAsMapWithKey godoc
// @Summary      按 key 列表查询选项 Map
// @Description  根据传入的 key 列表返回对应 key-value 形式的选项
// @Tags         Admin.Option
// @Accept       json
// @Produce      json
// @Param        keys  body     []string  true  "选项 key 列表"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=map[string]interface{}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/options/map_view/keys [post]
func (o *OptionHandler) ListAllOptionsAsMapWithKey(ctx *gin.Context) (interface{}, error) {
	keys := make([]string, 0)
	err := ctx.ShouldBindJSON(&keys)
	if err != nil {
		return nil, xerr.WithMsg(err, "option key error").WithStatus(xerr.StatusBadRequest)
	}
	options, err := o.OptionService.ListAllOption(ctx)
	if err != nil {
		return nil, err
	}
	keyMap := make(map[string]struct{})
	for _, key := range keys {
		keyMap[key] = struct{}{}
	}
	result := make(map[string]interface{})
	for _, option := range options {
		if _, ok := keyMap[option.Key]; ok {
			result[option.Key] = option.Value
		}
	}
	return result, nil
}

// SaveOptionWithMap godoc
// @Summary      以 Map 形式保存选项
// @Description  根据 key-value 映射批量保存配置选项
// @Tags         Admin.Option
// @Accept       json
// @Produce      json
// @Param        options  body     map[string]interface{}  true  "选项 key-value 映射"
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/options/map_view/saving [post]
func (o *OptionHandler) SaveOptionWithMap(ctx *gin.Context) (interface{}, error) {
	optionMap := make(map[string]interface{}, 0)
	err := ctx.ShouldBind(&optionMap)
	if err != nil {
		return nil, xerr.WithMsg(err, "parameter error").WithStatus(xerr.StatusBadRequest)
	}
	temp := make(map[string]string)
	for key, value := range optionMap {
		var v string
		switch value := value.(type) {
		case int32:
			v = strconv.Itoa(int(value))
		case int64:
			v = strconv.FormatInt(value, 10)
		case int:
			v = strconv.Itoa(value)
		case string:
			v = value
		case bool:
			v = strconv.FormatBool(value)
		case float64:
			v = strconv.FormatFloat(value, 'f', -1, 64)
		case float32:
			v = strconv.FormatFloat(float64(value), 'f', -1, 32)
		default:
			return nil, xerr.BadParam.New("key=%v,value=%v", key, value).WithStatus(xerr.StatusBadRequest).WithMsg("Parameter type is incorrect")
		}
		temp[key] = v
	}
	return nil, o.OptionService.Save(ctx, temp)
}
