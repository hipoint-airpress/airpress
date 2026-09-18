package vo

import "github.com/hipoint-airpress/airpress/model/dto"

type Menu struct {
	dto.Menu
	Children []*Menu `json:"children"`
}
