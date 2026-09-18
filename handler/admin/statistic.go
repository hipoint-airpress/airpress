package admin

import (
	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/service"
)

type StatisticHandler struct {
	StatisticService service.StatisticService
}

func NewStatisticHandler(l service.StatisticService) *StatisticHandler {
	return &StatisticHandler{
		StatisticService: l,
	}
}

// Statistics godoc
// @Summary      获取博客统计数据
// @Description  返回文章数、评论数、分类数、访问量等统计信息
// @Tags         Admin.Statistic
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.Statistic}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/statistics [get]
func (s *StatisticHandler) Statistics(ctx *gin.Context) (interface{}, error) {
	return s.StatisticService.Statistic(ctx)
}

// StatisticsWithUser godoc
// @Summary      获取含用户信息的统计数据
// @Description  返回博客统计信息,同时附带当前登录管理员信息
// @Tags         Admin.Statistic
// @Produce      json
// @Security     AdminApiKey
// @Success      200  {object}  dto.BaseDTO{data=dto.StatisticWithUser}
// @Failure      400  {object}  dto.BaseDTO
// @Failure      500  {object}  dto.BaseDTO
// @Router       /admin/statistics/user [get]
func (s *StatisticHandler) StatisticsWithUser(ctx *gin.Context) (interface{}, error) {
	return s.StatisticService.StatisticWithUser(ctx)
}
