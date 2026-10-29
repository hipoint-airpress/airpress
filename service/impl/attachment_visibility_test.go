package impl

import (
	"testing"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

func TestAttachmentPageIncludesAuthorPostAttachments(t *testing.T) {
	db := setupPostAttachmentDB(t)
	admin := createTransferUser(t, db, "admin", consts.UserRoleSuperAdmin)
	author := createTransferUser(t, db, "author", consts.UserRoleAuthor)
	other := createTransferUser(t, db, "other", consts.UserRoleAuthor)

	own := &entity.Attachment{Name: "own", Path: "upload/own.png", UserID: author.ID}
	linked := &entity.Attachment{Name: "linked", Path: "upload/linked.png", UserID: admin.ID}
	unrelated := &entity.Attachment{Name: "unrelated", Path: "upload/unrelated.png", UserID: other.ID}
	if err := db.Create([]*entity.Attachment{own, linked, unrelated}).Error; err != nil {
		t.Fatalf("create attachments: %v", err)
	}

	post := &entity.Post{Title: "p", Slug: "p", Type: consts.PostTypePost, AuthorID: author.ID}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}
	if err := db.Create(&entity.PostAttachment{PostID: post.ID, AttachmentID: linked.ID}).Error; err != nil {
		t.Fatalf("create relation: %v", err)
	}

	svc := &attachmentServiceImpl{}
	attachments, total, err := svc.Page(postActorContext(author), &param.AttachmentQuery{
		Page: param.Page{PageNum: 0, PageSize: 10},
	})
	if err != nil {
		t.Fatalf("Page() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}

	got := make(map[int32]bool, len(attachments))
	for _, attachment := range attachments {
		got[attachment.ID] = true
	}
	if !got[own.ID] {
		t.Fatal("own attachment should be visible")
	}
	if !got[linked.ID] {
		t.Fatal("attachment linked to author's post should be visible")
	}
	if got[unrelated.ID] {
		t.Fatal("unrelated attachment should not be visible")
	}
}

func TestAttachmentGetAllowsAuthorForLinkedAttachment(t *testing.T) {
	db := setupPostAttachmentDB(t)
	admin := createTransferUser(t, db, "admin", consts.UserRoleSuperAdmin)
	author := createTransferUser(t, db, "author", consts.UserRoleAuthor)
	other := createTransferUser(t, db, "other", consts.UserRoleAuthor)

	linked := &entity.Attachment{Name: "linked", Path: "upload/linked.png", UserID: admin.ID}
	unrelated := &entity.Attachment{Name: "unrelated", Path: "upload/unrelated.png", UserID: other.ID}
	if err := db.Create([]*entity.Attachment{linked, unrelated}).Error; err != nil {
		t.Fatalf("create attachments: %v", err)
	}

	post := &entity.Post{Title: "p", Slug: "p", Type: consts.PostTypePost, AuthorID: author.ID}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}
	if err := db.Create(&entity.PostAttachment{PostID: post.ID, AttachmentID: linked.ID}).Error; err != nil {
		t.Fatalf("create relation: %v", err)
	}

	svc := &attachmentServiceImpl{}
	got, err := svc.GetAttachment(postActorContext(author), linked.ID)
	if err != nil {
		t.Fatalf("GetAttachment() linked error = %v", err)
	}
	if got.ID != linked.ID {
		t.Fatalf("GetAttachment() id = %d, want %d", got.ID, linked.ID)
	}

	if _, err := svc.GetAttachment(postActorContext(author), unrelated.ID); err == nil {
		t.Fatal("author should not read unrelated attachment")
	} else if xerr.GetHTTPStatus(err) != xerr.StatusForbidden {
		t.Fatalf("status = %d, want %d", xerr.GetHTTPStatus(err), xerr.StatusForbidden)
	}
}

func TestSyncPostAttachmentsLinksNonAuthorAttachment(t *testing.T) {
	db := setupPostAttachmentDB(t)
	admin := createTransferUser(t, db, "admin", consts.UserRoleSuperAdmin)
	author := createTransferUser(t, db, "author", consts.UserRoleAuthor)

	attachment := &entity.Attachment{Name: "img", Path: "upload/image/2026/10/a.png", UserID: admin.ID}
	if err := db.Create(attachment).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	post := &entity.Post{
		Title:           "p",
		Slug:            "p",
		Type:            consts.PostTypePost,
		AuthorID:        author.ID,
		OriginalContent: "![a](https://blog.example.com/upload/image/2026/10/a.png)",
	}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}

	if _, err := (basePostServiceImpl{}).CreateOrUpdate(postActorContext(author), post, nil, nil, nil); err != nil {
		t.Fatalf("CreateOrUpdate: %v", err)
	}

	var count int64
	if err := db.Model(&entity.PostAttachment{}).
		Where("post_id = ? AND attachment_id = ?", post.ID, attachment.ID).
		Count(&count).Error; err != nil {
		t.Fatalf("count relations: %v", err)
	}
	if count != 1 {
		t.Fatalf("relation count = %d, want 1", count)
	}
}

