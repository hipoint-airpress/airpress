package service

import (
	"context"
	"io"

	"github.com/hipoint-airpress/airpress/model/entity"
)

type ExportImport interface {
	CreateByMarkdown(ctx context.Context, filename string, reader io.Reader) (*entity.Post, error)
	ExportMarkdown(ctx context.Context, needFrontMatter bool) (string, error)
}
