package admin

import (
	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type LogHandler struct {
	LogService service.LogService
}

func NewLogHandler(logService service.LogService) *LogHandler {
	return &LogHandler{
		LogService: logService,
	}
}

// PageLatestLog godoc
// @Summary      查询最新日志记录
// @Description  返回指定数量的最新系统日志列表
// @Tags         Admin.Log
// @Produce      json
// @Param        top  query     int  false  "返回数量"  example(10)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=[]dto.Log}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/logs/latest [get]
func (l *LogHandler) PageLatestLog(ctx *gin.Context) (interface{}, error) {
	top, err := util.MustGetQueryInt32(ctx, "top")
	if err != nil {
		top = 10
	}
	logs, _, err := l.LogService.PageLog(ctx, param.Page{PageSize: int(top)}, &param.Sort{Fields: []string{"createTime,desc"}})
	if err != nil {
		return nil, err
	}
	logDTOs := make([]*dto.Log, 0, len(logs))
	for _, log := range logs {
		logDTOs = append(logDTOs, l.LogService.ConvertToDTO(log))
	}
	return logDTOs, nil
}

// PageLog godoc
// @Summary      分页查询日志记录
// @Description  支持排序与分页的系统日志查询
// @Tags         Admin.Log
// @Accept       json
// @Produce      json
// @Param        page  query     int        false  "页码(从0开始)"  example(0)
// @Param        size  query     int        false  "每页数量"        example(10)
// @Param        sort  query     []string   false  "排序字段,如 createTime,desc"  collectionFormat(multi)
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Page{content=[]dto.Log}}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/logs [get]
func (l *LogHandler) PageLog(ctx *gin.Context) (interface{}, error) {
	type LogParam struct {
		param.Page
		*param.Sort
	}
	var logParam LogParam
	err := ctx.ShouldBindQuery(&logParam)
	if err != nil {
		return nil, xerr.WithMsg(err, "parameter error").WithStatus(xerr.StatusBadRequest)
	}
	if logParam.Sort == nil {
		logParam.Sort = &param.Sort{
			Fields: []string{"createTime,desc"},
		}
	}
	logs, totalCount, err := l.LogService.PageLog(ctx, logParam.Page, logParam.Sort)
	if err != nil {
		return nil, err
	}
	logDTOs := make([]*dto.Log, 0, len(logs))
	for _, log := range logs {
		logDTOs = append(logDTOs, l.LogService.ConvertToDTO(log))
	}
	return dto.NewPage(logDTOs, totalCount, logParam.Page), nil
}

// ClearLog godoc
// @Summary      清空日志
// @Description  清空所有系统日志记录
// @Tags         Admin.Log
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/logs/clear [get]
func (l *LogHandler) ClearLog(ctx *gin.Context) (interface{}, error) {
	return nil, l.LogService.Clear(ctx)
}
