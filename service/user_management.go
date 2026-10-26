package service

import (
	"context"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
)

type UserManagementService interface {
	List(ctx context.Context, query param.UserQuery) ([]*dto.User, int64, error)
	Get(ctx context.Context, id int32) (*dto.User, error)
	Create(ctx context.Context, userParam param.CreateUser) (*dto.User, error)
	Update(ctx context.Context, id int32, userParam param.UpdateUser) (*dto.User, error)
	UpdateRole(ctx context.Context, id int32, role consts.UserRole) error
	UpdateStatus(ctx context.Context, id int32, disabled bool) error
	ResetPassword(ctx context.Context, id int32, newPassword string) error
	ContentCount(ctx context.Context, id int32) (*dto.UserContentCount, error)
	Delete(ctx context.Context, id int32, transferToID int32) (*dto.UserContentCount, error)
	GetMFAPolicy(ctx context.Context) (consts.MFAPolicy, error)
	SaveMFAPolicy(ctx context.Context, policy consts.MFAPolicy) error
}
