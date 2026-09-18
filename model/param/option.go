package param

import "github.com/hipoint-airpress/airpress/consts"

type Option struct {
	Key   string `json:"key" form:"key"`
	Value string `json:"value" form:"value"`
}
type OptionQuery struct {
	Page
	Keyword string            `json:"keyword" form:"keyword"`
	Type    consts.OptionType `json:"type" form:"type"`
}
