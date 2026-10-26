package impl

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/cache"
	"github.com/hipoint-airpress/airpress/config"
	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/service"
)

var mfaPolicyDSNSeq int64

func setupMFAPolicyService(t *testing.T) service.OptionService {
	t.Helper()

	dsn := fmt.Sprintf("file:airpress_mfa_policy_%d?mode=memory&cache=shared",
		atomic.AddInt64(&mfaPolicyDSNSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&entity.Option{}); err != nil {
		t.Fatalf("auto migrate option: %v", err)
	}
	prev := *dal.Q
	t.Cleanup(func() {
		*dal.Q = prev
		_ = sqlDB.Close()
	})
	dal.SetDefault(db)

	return NewOptionService(&config.Config{}, cache.NewCache(), nil, zap.NewNop())
}

func TestGetMFAPolicyDefaultsToNone(t *testing.T) {
	svc := setupMFAPolicyService(t)

	policy, err := svc.GetMFAPolicy(context.Background())
	if err != nil {
		t.Fatalf("GetMFAPolicy() error = %v", err)
	}
	if policy != consts.MFAPolicyNone {
		t.Fatalf("GetMFAPolicy() = %v, want %v", policy, consts.MFAPolicyNone)
	}
}

func TestGetMFAPolicyReadsOption(t *testing.T) {
	svc := setupMFAPolicyService(t)

	option := &entity.Option{OptionKey: "mfa_policy", OptionValue: string(consts.MFAPolicyAll)}
	if err := dal.GetQueryByCtx(context.Background()).Option.WithContext(context.Background()).Create(option); err != nil {
		t.Fatalf("create option: %v", err)
	}

	policy, err := svc.GetMFAPolicy(context.Background())
	if err != nil {
		t.Fatalf("GetMFAPolicy() error = %v", err)
	}
	if policy != consts.MFAPolicyAll {
		t.Fatalf("GetMFAPolicy() = %v, want %v", policy, consts.MFAPolicyAll)
	}
}

func TestGetMFAPolicyFallsBackForUnknownValue(t *testing.T) {
	svc := setupMFAPolicyService(t)

	option := &entity.Option{OptionKey: "mfa_policy", OptionValue: "BOGUS"}
	if err := dal.GetQueryByCtx(context.Background()).Option.WithContext(context.Background()).Create(option); err != nil {
		t.Fatalf("create option: %v", err)
	}

	policy, err := svc.GetMFAPolicy(context.Background())
	if err != nil {
		t.Fatalf("GetMFAPolicy() error = %v", err)
	}
	if policy != consts.MFAPolicyNone {
		t.Fatalf("GetMFAPolicy() = %v, want %v", policy, consts.MFAPolicyNone)
	}
}
