package model

import (
	"context"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/template"
)

func NewJournalModel(optionService service.OptionService,
	themeService service.ThemeService,
	journalService service.JournalService,
) *JournalModel {
	return &JournalModel{
		OptionService:  optionService,
		ThemeService:   themeService,
		JournalService: journalService,
	}
}

type JournalModel struct {
	JournalService service.JournalService
	OptionService  service.OptionService
	ThemeService   service.ThemeService
}

func (p *JournalModel) Journals(ctx context.Context, model template.Model, page int) (string, error) {
	pageSize := p.OptionService.GetOrByDefault(ctx, property.JournalPageSize).(int)
	journalType := consts.JournalTypePublic
	journals, total, err := p.JournalService.ListJournal(ctx, param.JournalQuery{
		Page: param.Page{PageNum: page, PageSize: pageSize},
		Sort: &param.Sort{
			Fields: []string{"createTime,desc"},
		},
		Keyword:     nil,
		JournalType: &journalType,
	})
	if err != nil {
		return "", err
	}
	journalDTOs, err := p.JournalService.ConvertToWithCommentDTOList(ctx, journals)
	if err != nil {
		return "", err
	}
	journalPage := dto.NewPage(journalDTOs, total, param.Page{PageNum: page, PageSize: pageSize})
	model["is_journals"] = true
	model["journals"] = journalPage
	model["meta_keywords"] = p.OptionService.GetOrByDefault(ctx, property.SeoKeywords)
	model["meta_description"] = p.OptionService.GetOrByDefault(ctx, property.SeoDescription)
	return p.ThemeService.Render(ctx, "journals")
}
