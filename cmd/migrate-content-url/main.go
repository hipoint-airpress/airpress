package main

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/fx"

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
		fx.Invoke(func() error {
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
