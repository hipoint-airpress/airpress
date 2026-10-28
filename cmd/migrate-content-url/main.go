package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/config"
	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/log"
	serviceimpl "github.com/hipoint-airpress/airpress/service/impl"
)

func main() {
	app := fx.New(
		fx.NopLogger,
		fx.Provide(log.NewLogger),
		fx.Provide(log.NewGormLogger),
		fx.Provide(dal.NewGormDB),
		fx.Provide(config.NewConfig),
		fx.Invoke(func(cfg *config.Config, _ *gorm.DB) error {
			fmt.Printf("target database: %s\n", databaseTarget(cfg))
			fmt.Println("migrating content urls...")
			updated, err := serviceimpl.MigrateContentURLs(context.Background())
			if err != nil {
				return err
			}
			fmt.Printf("migrated %d records\n", updated)
			return nil
		}),
	)
	if err := app.Start(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "migrate content urls failed:", err)
		os.Exit(1)
	}
	if err := app.Stop(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "stop app failed:", err)
		os.Exit(1)
	}
}

// databaseTarget 返回迁移目标的脱敏描述，便于执行前确认没有误连其他数据库。
// 与 dal.NewGormDB 的优先级保持一致：SQLite3 优先于 MySQL。
func databaseTarget(cfg *config.Config) string {
	if cfg.SQLite3 != nil && cfg.SQLite3.Enable {
		return "sqlite:" + cfg.SQLite3.File
	}
	if cfg.MySQL != nil {
		return "mysql:" + mysqlTarget(cfg.MySQL.Dsn)
	}
	return "unknown"
}

// mysqlTarget 从 DSN 中提取 host 与 dbname，避免输出账号密码。
// 支持 tcp(host:port)/dbname 与 unix(/path)/dbname 两种形式。
func mysqlTarget(dsn string) string {
	core := dsn
	if i := strings.IndexByte(core, '?'); i >= 0 {
		core = core[:i]
	}
	slash := strings.LastIndexByte(core, '/')
	authority := core
	dbName := ""
	if slash >= 0 {
		authority = core[:slash]
		dbName = core[slash+1:]
	}
	if at := strings.LastIndexByte(authority, '@'); at >= 0 {
		authority = authority[at+1:]
	}
	host := authority
	if open := strings.IndexByte(authority, '('); open >= 0 {
		if close := strings.LastIndexByte(authority, ')'); close > open {
			host = authority[open+1 : close]
		}
	}
	if host == "" {
		host = "unknown"
	}
	if dbName == "" {
		dbName = "unknown"
	}
	return host + "/" + dbName
}
