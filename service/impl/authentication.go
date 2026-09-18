package impl

import (
	"context"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

func GetAuthorizedUser(ctx context.Context) (*entity.User, bool) {
	user, ok := ctx.Value(consts.AuthorizedUser).(*entity.User)
	if !ok {
		return nil, false
	}
	return user, true
}

func MustGetAuthorizedUser(ctx context.Context) (*entity.User, error) {
	user, ok := ctx.Value(consts.AuthorizedUser).(*entity.User)
	if !ok || user == nil {
		return nil, xerr.WithStatus(nil, xerr.StatusForbidden).WithMsg("Not logged in")
	}
	return user, nil
}
