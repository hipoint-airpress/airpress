package dal

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/hipoint-airpress/airpress/config"
	"github.com/hipoint-airpress/airpress/consts"
	airpressLog "github.com/hipoint-airpress/airpress/log"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

var (
	DB     *gorm.DB
	DBType consts.DBType
)

func NewGormDB(conf *config.Config, gormLogger logger.Interface) *gorm.DB {
	var err error

	//nolint:gocritic
	if conf.SQLite3 != nil && conf.SQLite3.Enable {
		DB, err = initSQLite(conf, gormLogger)
		if err != nil {
			airpressLog.Fatal("open SQLite3 error", zap.Error(err))
		}
		DBType = consts.DBTypeSQLite
	} else if conf.MySQL != nil {
		DB, err = initMySQL(conf, gormLogger)
		if err != nil {
			airpressLog.Fatal("connect to MySQL error", zap.Error(err))
		}
		DBType = consts.DBTypeMySQL
	} else {
		airpressLog.Fatal("No database available")
	}
	if DB == nil {
		airpressLog.Fatal("no available database")
	}
	airpressLog.Info("connect database success")
	sqlDB, err := DB.DB()
	if err != nil {
		airpressLog.Fatal("get database connection error")
	}
	sqlDB.SetMaxIdleConns(200)
	sqlDB.SetMaxOpenConns(300)
	sqlDB.SetConnMaxIdleTime(time.Hour)
	SetDefault(DB)
	dbMigrate()
	return DB
}

func initMySQL(conf *config.Config, gormLogger logger.Interface) (*gorm.DB, error) {
	mysqlConfig := conf.MySQL
	if mysqlConfig == nil {
		return nil, xerr.WithMsg(nil, "nil MySQL config")
	}
	dsn := mysqlConfig.Dsn

	airpressLog.Info("try connect to MySQL", zap.String("dsn", `Use dsn in config`))
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger:                   gormLogger,
		PrepareStmt:              true,
		SkipDefaultTransaction:   true,
		DisableNestedTransaction: true,
	})
	return db, err
}

func initSQLite(conf *config.Config, gormLogger logger.Interface) (*gorm.DB, error) {
	sqliteConfig := conf.SQLite3
	if sqliteConfig == nil {
		return nil, xerr.WithMsg(nil, "nil SQLite config")
	}
	airpressLog.Info("try to open SQLite3 db", zap.String("path", sqliteConfig.File))
	db, err := gorm.Open(sqlite.Open(sqliteConfig.File), &gorm.Config{
		Logger:                   gormLogger,
		PrepareStmt:              true,
		SkipDefaultTransaction:   true,
		DisableNestedTransaction: true,
	})
	return db, err
}

func dbMigrate() {
	db := DB.Session(&gorm.Session{
		Logger: DB.Logger.LogMode(logger.Warn),
	})
	err := db.AutoMigrate(&entity.Attachment{}, &entity.Category{}, &entity.Comment{}, &entity.CommentBlack{}, &entity.Journal{},
		&entity.Link{}, &entity.Log{}, &entity.Menu{}, &entity.Meta{}, &entity.Option{}, &entity.Photo{}, &entity.Post{},
		&entity.PostCategory{}, &entity.PostTag{}, &entity.Tag{}, &entity.ThemeSetting{}, &entity.User{})
	if err != nil {
		airpressLog.Fatal("failed auto migrate db", zap.Error(err))
	}
}

type ctxTransaction struct{}

func GetQueryByCtx(ctx context.Context) *Query {
	dbI := ctx.Value(ctxTransaction{})

	if dbI != nil {
		db, ok := dbI.(*Query)
		if !ok {
			panic("unexpected context query value type")
		}
		if db != nil {
			return db
		}
	}
	return Q
}

func SetCtxQuery(ctx context.Context, q *Query) context.Context {
	return context.WithValue(ctx, ctxTransaction{}, q)
}

func Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	q := GetQueryByCtx(ctx)
	return q.Transaction(func(tx *Query) error {
		txCtx := SetCtxQuery(ctx, tx)
		return fn(txCtx)
	})
}

func GetDB() *gorm.DB {
	return DB
}
