package vo

import "github.com/hipoint-airpress/airpress/model/dto"

type LinkTeamVO struct {
	Team  string
	Links []*dto.Link
}
