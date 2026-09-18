package listener

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/event"
	"github.com/hipoint-airpress/airpress/log"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
)

type StartListener struct {
	db            *gorm.DB
	optionService service.OptionService
	bus           event.Bus
}

func NewStartListener(db *gorm.DB, optionService service.OptionService, bus event.Bus) {
	s := StartListener{
		db:            db,
		optionService: optionService,
		bus:           bus,
	}
	bus.Subscribe(event.StartEventName, s.HandleEvent)
}

func (s *StartListener) HandleEvent(ctx context.Context, startEvent event.Event) error {
	if _, ok := startEvent.(*event.StartEvent); !ok {
		return nil
	}

	err := s.createOptions()
	if err != nil {
		log.Error("create options err", zap.Error(err))
	}
	switch dal.DBType {
	case consts.DBTypeMySQL:
		err = dal.DB.Session(&gorm.Session{Context: ctx}).Raw("SELECT VERSION()").Scan(&consts.DatabaseVersion).Error
	case consts.DBTypeSQLite:
		err = dal.DB.Session(&gorm.Session{Context: ctx}).Raw("SELECT SQLITE_VERSION()").Scan(&consts.DatabaseVersion).Error
	}
	if err != nil {
		return err
	}
	_ = s.printStartInfo(ctx)
	return nil
}

func (s *StartListener) createOptions() error {
	ctx := context.Background()

	ctx = dal.SetCtxQuery(ctx, dal.GetQueryByCtx(ctx).ReplaceDB(dal.GetDB().Session(
		&gorm.Session{Logger: dal.DB.Logger.LogMode(logger.Warn)},
	)))

	optionDAL := dal.GetQueryByCtx(ctx).Option
	options, err := optionDAL.WithContext(ctx).Find()
	if err != nil {
		return err
	}
	toCreate := make([]*entity.Option, 0)
out:
	for _, p := range property.AllProperty {
		for _, o := range options {
			if p.KeyValue == o.OptionKey {
				continue out
			}
		}
		toCreate = append(toCreate, p.ConvertToOption())
	}
	return optionDAL.WithContext(ctx).Create(toCreate...)
}

func (s *StartListener) printStartInfo(ctx context.Context) error {
	blogURL, err := s.optionService.GetBlogBaseURL(ctx)
	if err != nil {
		return err
	}
	site := logger.BlueBold + "AirPress started at      " + blogURL + logger.Reset
	log.Info(site)
	fmt.Println(site)

	adminURLPath, err := s.optionService.GetAdminURLPath(ctx)
	if err != nil {
		return err
	}
	adminSite := logger.BlueBold + "AirPress admin started at      " + blogURL + "/" + adminURLPath + logger.Reset
	log.Info(adminSite)
	fmt.Println(adminSite)
	return nil
}
