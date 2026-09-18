package vo

import "github.com/hipoint-airpress/airpress/model/dto"

type CategoryVO struct {
	dto.CategoryDTO
	Children []*CategoryVO `json:"children"`
}
