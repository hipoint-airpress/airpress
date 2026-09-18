package theme

import (
	"context"

	"github.com/hipoint-airpress/airpress/model/dto"
)

type ThemeFetcher interface {
	FetchTheme(ctx context.Context, file interface{}) (*dto.ThemeProperty, error)
}
