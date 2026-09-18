package service

import (
	"context"

	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/model/vo"
)

type CategoryService interface {
	GetByID(ctx context.Context, id int32) (*entity.Category, error)
	GetBySlug(ctx context.Context, slug string) (*entity.Category, error)
	GetByName(ctx context.Context, name string) (*entity.Category, error)
	ListCategoryWithPostCountDTO(ctx context.Context, sort *param.Sort) ([]*dto.CategoryWithPostCount, error)
	ListAll(ctx context.Context, sort *param.Sort) ([]*entity.Category, error)
	ConvertToCategoryDTO(ctx context.Context, e *entity.Category) (*dto.CategoryDTO, error)
	ConvertToCategoryDTOs(ctx context.Context, categories []*entity.Category) ([]*dto.CategoryDTO, error)
	ListAsTree(ctx context.Context, sort *param.Sort, fillPassword bool) ([]*vo.CategoryVO, error)
	Create(ctx context.Context, categoryParam *param.Category) (*entity.Category, error)
	Update(ctx context.Context, categoryParam *param.Category) (*entity.Category, error)
	UpdateBatch(ctx context.Context, categoryParams []*param.Category) ([]*entity.Category, error)
	Delete(ctx context.Context, categoryID int32) error
	ListByIDs(ctx context.Context, categoryIDs []int32) ([]*entity.Category, error)
	IsCategoriesEncrypt(ctx context.Context, categoryIDs ...int32) (bool, error)
	Count(ctx context.Context) (int64, error)
	GetChildCategory(ctx context.Context, parentCategoryID int32) ([]*entity.Category, error)
}
