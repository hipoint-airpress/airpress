package model

import (
	"context"
	"strings"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/handler/content/authentication"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/service/assembler"
	"github.com/hipoint-airpress/airpress/template"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

func NewSheetModel(optionService service.OptionService,
	themeService service.ThemeService,
	postTagService service.PostTagService,
	tagService service.TagService,
	metaService service.MetaService,
	sheetAssembler assembler.SheetAssembler,
	sheetService service.SheetService,
	postAuthentication *authentication.PostAuthentication,
) *SheetModel {
	return &SheetModel{
		OptionService:      optionService,
		ThemeService:       themeService,
		PostTagService:     postTagService,
		TagService:         tagService,
		MetaService:        metaService,
		SheetAssembler:     sheetAssembler,
		SheetService:       sheetService,
		PostAuthentication: postAuthentication,
	}
}

type SheetModel struct {
	SheetService       service.SheetService
	OptionService      service.OptionService
	ThemeService       service.ThemeService
	PostTagService     service.PostTagService
	TagService         service.TagService
	MetaService        service.MetaService
	SheetAssembler     assembler.SheetAssembler
	PostAuthentication *authentication.PostAuthentication
}

// resolveSheetTemplate 返回页面详情应使用的模板名：优先使用页面的自定义模板（sheet.template），
// 若该字段未设置或对应模板文件不存在，则回退到默认 sheet 模板，避免渲染因模板缺失而报错。
func (s *SheetModel) resolveSheetTemplate(ctx context.Context, sheet *entity.Post) string {
	if sheet.Template == "" {
		return "sheet"
	}
	// template 存短名（与 console 下拉选项、ListCustomTemplates 一致），
	// 主题内自定义模板文件按 <sheet_ 前缀><短名>.tmpl 命名；TrimSuffix 兜底历史脏数据。
	name := consts.ThemeCustomSheetPrefix + strings.TrimSuffix(sheet.Template, ".tmpl")
	exist, err := s.ThemeService.TemplateExist(ctx, name+".tmpl")
	if err != nil || !exist {
		return "sheet"
	}
	return name
}

func (s *SheetModel) Content(ctx context.Context, sheet *entity.Post, token string, model template.Model) (string, error) {
	if sheet == nil {
		return "", xerr.WithStatus(nil, int(xerr.StatusBadRequest)).WithMsg("查询不到文章信息")
	}
	switch sheet.Status {
	case consts.PostStatusRecycle, consts.PostStatusDraft:
		return "", xerr.WithStatus(nil, xerr.StatusNotFound).WithMsg("查询不到文章信息")
	case consts.PostStatusIntimate:
		if isAuthenticated, err := s.PostAuthentication.IsAuthenticated(ctx, token, sheet.ID); err != nil || !isAuthenticated {
			model["slug"] = sheet.Slug
			model["type"] = consts.EncryptTypePost.Name()
			if exist, err := s.ThemeService.TemplateExist(ctx, "post_password.tmpl"); err == nil && exist {
				return s.ThemeService.Render(ctx, "post_password")
			}
			return "common/template/post_password", nil
		}
	}

	sheetVO, err := s.SheetAssembler.ConvertToDetailVO(ctx, sheet)
	if err != nil {
		return "", err
	}
	model["target"] = sheetVO
	model["type"] = "sheet"
	model["post"] = sheetVO
	model["sheet"] = sheetVO
	model["is_sheet"] = true

	metas, err := s.MetaService.GetPostMeta(ctx, sheet.ID)
	if err != nil {
		return "", err
	}
	model["metas"] = s.MetaService.ConvertToMetaDTOs(metas)

	tags, err := s.PostTagService.ListTagByPostID(ctx, sheet.ID)
	if err != nil {
		return "", err
	}
	model["tags"], _ = s.TagService.ConvertToDTOs(ctx, tags)

	if sheet.MetaDescription != "" {
		model["meta_description"] = sheet.MetaDescription
	} else {
		model["meta_description"] = sheet.Summary
	}
	if sheet.MetaKeywords != "" {
		model["meta_keywords"] = sheet.MetaKeywords
	} else if len(tags) > 0 {
		metaKeywords := strings.Builder{}
		metaKeywords.Write([]byte(tags[0].Name))
		for _, tag := range tags[1:] {
			metaKeywords.Write([]byte(","))
			metaKeywords.Write([]byte(tag.Name))
		}
		model["meta_keywords"] = metaKeywords.String()
	}

	s.SheetService.IncreaseVisit(ctx, sheet.ID)

	return s.ThemeService.Render(ctx, s.resolveSheetTemplate(ctx, sheet))
}

func (s *SheetModel) AdminPreviewContent(ctx context.Context, sheet *entity.Post, model template.Model) (string, error) {
	if sheet == nil {
		return "", xerr.WithStatus(nil, int(xerr.StatusBadRequest)).WithMsg("查询不到文章信息")
	}

	sheetVO, err := s.SheetAssembler.ConvertToDetailVO(ctx, sheet)
	if err != nil {
		return "", err
	}
	model["target"] = sheetVO
	model["type"] = "sheet"
	model["post"] = sheetVO
	model["sheet"] = sheetVO
	model["is_sheet"] = true

	metas, err := s.MetaService.GetPostMeta(ctx, sheet.ID)
	if err != nil {
		return "", err
	}
	model["metas"] = s.MetaService.ConvertToMetaDTOs(metas)

	tags, err := s.PostTagService.ListTagByPostID(ctx, sheet.ID)
	if err != nil {
		return "", err
	}
	model["tags"], _ = s.TagService.ConvertToDTOs(ctx, tags)

	if sheet.MetaDescription != "" {
		model["meta_description"] = sheet.MetaDescription
	} else {
		model["meta_description"] = sheet.Summary
	}
	if sheet.MetaKeywords != "" {
		model["meta_keywords"] = sheet.MetaKeywords
	} else if len(tags) > 0 {
		metaKeywords := strings.Builder{}
		metaKeywords.Write([]byte(tags[0].Name))
		for _, tag := range tags[1:] {
			metaKeywords.Write([]byte(","))
			metaKeywords.Write([]byte(tag.Name))
		}
		model["meta_keywords"] = metaKeywords.String()
	}

	return s.ThemeService.Render(ctx, s.resolveSheetTemplate(ctx, sheet))
}
