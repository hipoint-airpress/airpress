package param

import "github.com/hipoint-airpress/airpress/consts"

type AttachmentQuery struct {
	Page
	Keyword        string                 `json:"keyword" form:"keyword"`
	MediaType      string                 `json:"mediaType" form:"mediaType"`
	AttachmentType *consts.AttachmentType `json:"attachmentType" form:"attachmentType"`
}

type AttachmentUpdate struct {
	Name string `json:"name" binding:"gte=1,lte=255"`
}
