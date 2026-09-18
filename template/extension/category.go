package extension

import (
	"context"

	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/model/vo"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/template"
)

type categoryExtension struct {
	Template            *template.Template
	CategoryService     service.CategoryService
	PostCategoryService service.PostCategoryService
}

func RegisterCategoryFunc(t *template.Template, categoryService service.CategoryService, postCategoryService service.PostCategoryService) {
	ce := &categoryExtension{
		Template:            t,
		CategoryService:     categoryService,
		PostCategoryService: postCategoryService,
	}
	ce.addListCategoryFunc()
	ce.addListCategoryAsTreeFunc()
	ce.addGetCategoryCountFunc()
	ce.addListCategoryByPostIDFunc()
	ce.addGetCategoryBySlugFunc()
}

func (ce *categoryExtension) addListCategoryFunc() {
	listCategory := func() ([]*dto.CategoryWithPostCount, error) {
		sort := param.Sort{
			Fields: []string{"priority,asc"},
		}
		return ce.CategoryService.ListCategoryWithPostCountDTO(context.Background(), &sort)
	}
	ce.Template.AddFunc("listCategory", listCategory)
}

func (ce *categoryExtension) addListCategoryAsTreeFunc() {
	listCategoryAsTree := func() ([]*vo.CategoryVO, error) {
		sort := param.Sort{
			Fields: []string{"priority,asc"},
		}
		return ce.CategoryService.ListAsTree(context.Background(), &sort, false)
	}
	ce.Template.AddFunc("listCategoryAsTree", listCategoryAsTree)
}

func (ce *categoryExtension) addListCategoryByPostIDFunc() {
	listCategoryByPostID := func(postID int) ([]*dto.CategoryDTO, error) {
		categories, err := ce.PostCategoryService.ListCategoryByPostID(context.Background(), int32(postID))
		if err != nil {
			return nil, err
		}
		return ce.CategoryService.ConvertToCategoryDTOs(context.Background(), categories)
	}
	ce.Template.AddFunc("listCategoryByPostID", listCategoryByPostID)
}

func (ce *categoryExtension) addGetCategoryCountFunc() {
	getCategoryCount := func() (int64, error) {
		return ce.CategoryService.Count(context.Background())
	}
	ce.Template.AddFunc("getCategoryCount", getCategoryCount)
}

// addGetCategoryBySlugFunc 按 slug 查找分类。
// 供主题模板按配置项（如 settings.beginner_category_slug）定位特定分类，
// pongo2 的 {% set %} 在循环体内写子作用域、循环外不可见，无法沿用「遍历查找并赋值」的写法。
func (ce *categoryExtension) addGetCategoryBySlugFunc() {
	getCategoryBySlug := func(slug string) (*dto.CategoryWithPostCount, error) {
		sort := param.Sort{
			Fields: []string{"priority,asc"},
		}
		categories, err := ce.CategoryService.ListCategoryWithPostCountDTO(context.Background(), &sort)
		if err != nil {
			return nil, err
		}
		for _, category := range categories {
			if category.CategoryDTO != nil && category.Slug == slug {
				return category, nil
			}
		}
		// 未匹配返回 nil，由模板的 {% if %} 兜底
		return nil, nil
	}
	ce.Template.AddFunc("getCategoryBySlug", getCategoryBySlug)
}
