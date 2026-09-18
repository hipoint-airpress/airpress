package extension

import (
	"context"

	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/template"
)

type statisticExtension struct {
	Template         *template.Template
	StatisticService service.StatisticService
}

func RegisterStatisticFunc(template *template.Template, statisticService service.StatisticService) {
	s := &statisticExtension{
		Template:         template,
		StatisticService: statisticService,
	}
	s.addGetStatisticsData()
}

func (s *statisticExtension) addGetStatisticsData() {
	getStatisticsDataFunc := func() (*dto.Statistic, error) {
		ctx := context.Background()
		statistic, err := s.StatisticService.Statistic(ctx)
		if err != nil {
			return nil, err
		}
		return statistic, nil
	}
	s.Template.AddFunc("getStatisticsData", getStatisticsDataFunc)
}
