package model

import (
	"context"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/service/assembler"
	"github.com/hipoint-airpress/airpress/template"
)

func NewTagModel(optionService service.OptionService,
	themeService service.ThemeService,
	tagService service.TagService,
	postTagService service.PostTagService,
	postAssembler assembler.PostAssembler,
) *TagModel {
	return &TagModel{
		OptionService:  optionService,
		ThemeService:   themeService,
		TagService:     tagService,
		PostAssembler:  postAssembler,
		PostTagService: postTagService,
	}
}

type TagModel struct {
	TagService     service.TagService
	OptionService  service.OptionService
	ThemeService   service.ThemeService
	PostTagService service.PostTagService
	MetaService    service.MetaService
	PostAssembler  assembler.PostAssembler
}

func (t *TagModel) Tags(ctx context.Context, model template.Model) (string, error) {
	model["is_tags"] = true
	model["meta_keywords"] = t.OptionService.GetOrByDefault(ctx, property.SeoKeywords)
	model["meta_description"] = t.OptionService.GetOrByDefault(ctx, property.SeoDescription)
	return t.ThemeService.Render(ctx, "tags")
}

func (t *TagModel) TagPosts(ctx context.Context, model template.Model, slug string, page int) (string, error) {
	tag, err := t.TagService.GetBySlug(ctx, slug)
	if err != nil {
		return "", err
	}
	tagDTO, err := t.TagService.ConvertToDTO(ctx, tag)
	if err != nil {
		return "", err
	}
	pageSize := t.OptionService.GetOrByDefault(ctx, property.ArchivePageSize).(int)
	posts, totalPage, err := t.PostTagService.PagePost(ctx, param.PostQuery{
		Page: param.Page{
			PageNum:  page,
			PageSize: pageSize,
		},
		Sort: &param.Sort{
			Fields: []string{"createTime,desc"},
		},
		Statuses: []*consts.PostStatus{consts.PostStatusPublished.Ptr()},
		TagID:    &tag.ID,
	})
	if err != nil {
		return "", err
	}
	postVOs, err := t.PostAssembler.ConvertToListVO(ctx, posts)
	if err != nil {
		return "", err
	}
	postPage := dto.NewPage(postVOs, totalPage, param.Page{
		PageNum:  page,
		PageSize: pageSize,
	})
	model["is_tag"] = true
	model["posts"] = postPage
	model["tag"] = tagDTO
	model["meta_keywords"] = t.OptionService.GetOrByDefault(ctx, property.SeoKeywords)
	model["meta_description"] = t.OptionService.GetOrByDefault(ctx, property.SeoDescription)
	return t.ThemeService.Render(ctx, "tag")
}
