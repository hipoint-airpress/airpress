package vo

import "github.com/hipoint-airpress/airpress/model/dto"

type SheetDetail struct {
	dto.PostDetail
	MetaIDs []int32     `json:"metaIds"`
	Metas   []*dto.Meta `json:"metas"`
}

type SheetList struct {
	dto.Post
	CommentCount int64 `json:"commentCount"`
}
