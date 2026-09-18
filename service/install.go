package service

import (
	"context"

	"github.com/hipoint-airpress/airpress/model/param"
)

type InstallService interface {
	InstallBlog(ctx context.Context, installParam param.Install) error
}
