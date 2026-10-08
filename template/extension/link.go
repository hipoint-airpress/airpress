package extension

import (
	"context"
	"math/rand"

	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/template"
)

type linkExtension struct {
	LinkService service.LinkService
	Template    *template.Template
}

// LinkTeam 按分组聚合的友情链接，供主题模板以 team.Team / team.Links 直接遍历。
type LinkTeam struct {
	Team  string
	Links []*dto.Link
}

func RegisterLinkFunc(template *template.Template, linkService service.LinkService) {
	l := &linkExtension{
		LinkService: linkService,
		Template:    template,
	}
	l.addListLinks()
	l.addGetLinksCount()
	l.addListLinksGroupByTeam()
	l.addListLinksRandom()
}

func (l *linkExtension) addListLinks() {
	listLinks := func() ([]*dto.Link, error) {
		ctx := context.Background()
		links, err := l.LinkService.List(ctx, nil)
		if err != nil {
			return nil, err
		}
		return l.LinkService.ConvertToDTOs(ctx, links), nil
	}
	l.Template.AddFunc("listLinks", listLinks)
}

func (l *linkExtension) addListLinksRandom() {
	listLinksRandom := func() ([]*dto.Link, error) {
		ctx := context.Background()
		links, err := l.LinkService.List(ctx, nil)
		if err != nil {
			return nil, err
		}
		rand.Shuffle(len(links), func(i, j int) {
			links[i], links[j] = links[j], links[i]
		})
		return l.LinkService.ConvertToDTOs(ctx, links), nil
	}
	l.Template.AddFunc("listLinksRandom", listLinksRandom)
}

// addListLinksGroupByTeam 注册 listLinksGroupByTeam：按 Team 分组并保持稳定顺序。
//
// 出参为切片而非 map（map 遍历顺序随机），模板可写成
//
//	{% for team in listLinksGroupByTeam() %}{{ team.Team }}{% for link in team.Links %}...
func (l *linkExtension) addListLinksGroupByTeam() {
	listLinksGroupByTeam := func() ([]LinkTeam, error) {
		ctx := context.Background()
		links, err := l.LinkService.List(ctx, nil)
		if err != nil {
			return nil, err
		}
		linkDTOs := l.LinkService.ConvertToDTOs(ctx, links)

		// 保持数据库返回的先后顺序：分组按首次出现的次序，组内保持原次序
		index := make(map[string]int)
		teams := make([]LinkTeam, 0)
		for _, link := range linkDTOs {
			i, ok := index[link.Team]
			if !ok {
				i = len(teams)
				index[link.Team] = i
				teams = append(teams, LinkTeam{Team: link.Team})
			}
			teams[i].Links = append(teams[i].Links, link)
		}
		return teams, nil
	}
	l.Template.AddFunc("listLinksGroupByTeam", listLinksGroupByTeam)
}

func (l *linkExtension) addGetLinksCount() {
	getLinksCount := func() (int64, error) {
		ctx := context.Background()
		return l.LinkService.Count(ctx)
	}
	l.Template.AddFunc("getLinksCount", getLinksCount)
}
