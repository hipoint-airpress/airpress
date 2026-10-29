package dto

import (
	"github.com/hipoint-airpress/airpress/consts"
)

type AttachmentDTO struct {
	ID             int32                 `json:"id"`
	Name           string                `json:"name"`
	Path           string                `json:"path"`
	FileKey        string                `json:"fileKey"`
	ThumbPath      string                `json:"thumbPath"`
	MediaType      string                `json:"mediaType"`
	Suffix         string                `json:"suffix"`
	Width          int32                 `json:"width"`
	Height         int32                 `json:"height"`
	Size           int64                 `json:"size"`
	AttachmentType consts.AttachmentType `json:"type"`
	// Owner 是附件归属人，附件无归属或用户已删除时为空。
	Owner *AttachmentOwner `json:"owner"`
}

type AttachmentOwner struct {
	ID       int32  `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}
