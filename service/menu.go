package service

import (
	"context"

	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/model/vo"
)

type MenuService interface {
	List(ctx context.Context, sort *param.Sort) ([]*entity.Menu, error)
	ListByTeam(ctx context.Context, team string, sort *param.Sort) ([]*entity.Menu, error)
	ListAsTree(ctx context.Context, sort *param.Sort) ([]*vo.Menu, error)
	ListAsTreeByTeam(ctx context.Context, team string, sort *param.Sort) ([]*vo.Menu, error)
	GetByID(ctx context.Context, id int32) (*entity.Menu, error)
	Create(ctx context.Context, menuParam *param.Menu) (*entity.Menu, error)
	CreateBatch(ctx context.Context, menuParams []*param.Menu) ([]*entity.Menu, error)
	Update(ctx context.Context, id int32, menuParam *param.Menu) (*entity.Menu, error)
	UpdateBatch(ctx context.Context, menuParams []*param.Menu) ([]*entity.Menu, error)
	Delete(ctx context.Context, id int32) error
	DeleteBatch(ctx context.Context, ids []int32) error
	ConvertToDTO(ctx context.Context, menu *entity.Menu) *dto.Menu
	ConvertToDTOs(ctx context.Context, menus []*entity.Menu) []*dto.Menu
	ListTeams(ctx context.Context) ([]string, error)
	GetMenuCount(ctx context.Context) (int64, error)
}
