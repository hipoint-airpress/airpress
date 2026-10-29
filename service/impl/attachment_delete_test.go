package impl

import (
	"testing"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

func newAttachmentDeleteService() *attachmentServiceImpl {
	return &attachmentServiceImpl{
		FileStorageComposite: &stubStorageComposite{storage: &stubFileStorage{}},
	}
}

func TestDeleteAttachmentReturnsReferencedPosts(t *testing.T) {
	db := setupPostAttachmentDB(t)
	editor := createTransferUser(t, db, "editor", consts.UserRoleEditor)
	attachment := &entity.Attachment{Name: "img", Path: "upload/image/2026/10/a.png", UserID: editor.ID, FileKey: "k"}
	if err := db.Create(attachment).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	for _, title := range []string{"文章一", "文章二"} {
		post := &entity.Post{Title: title, Slug: title, Type: consts.PostTypePost, AuthorID: editor.ID}
		if err := db.Create(post).Error; err != nil {
			t.Fatalf("create post: %v", err)
		}
		if err := db.Create(&entity.PostAttachment{PostID: post.ID, AttachmentID: attachment.ID}).Error; err != nil {
			t.Fatalf("create relation: %v", err)
		}
	}

	svc := newAttachmentDeleteService()
	_, err := svc.Delete(postActorContext(editor), attachment.ID, false)
	if err == nil || xerr.GetHTTPStatus(err) != xerr.StatusConflict {
		t.Fatalf("Delete() error = %v, want 409", err)
	}
	data, ok := xerr.GetData(err).(*dto.AttachmentReferenceData)
	if !ok || data == nil {
		t.Fatalf("data = %#v, want AttachmentReferenceData", xerr.GetData(err))
	}
	if len(data.Posts) != 2 {
		t.Fatalf("referenced posts = %d, want 2", len(data.Posts))
	}
}

func TestDeleteAttachmentAuthorCannotDeleteOthers(t *testing.T) {
	db := setupPostAttachmentDB(t)
	owner := createTransferUser(t, db, "owner", consts.UserRoleAuthor)
	other := createTransferUser(t, db, "other", consts.UserRoleAuthor)
	attachment := &entity.Attachment{Name: "img", Path: "upload/a.png", UserID: owner.ID, FileKey: "k"}
	if err := db.Create(attachment).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	svc := newAttachmentDeleteService()
	_, err := svc.Delete(postActorContext(other), attachment.ID, false)
	if err == nil || xerr.GetHTTPStatus(err) != xerr.StatusForbidden {
		t.Fatalf("Delete() error = %v, want 403", err)
	}
}

func TestDeleteAttachmentAuthorCannotForce(t *testing.T) {
	db := setupPostAttachmentDB(t)
	author := createTransferUser(t, db, "author", consts.UserRoleAuthor)
	attachment := &entity.Attachment{Name: "img", Path: "upload/a.png", UserID: author.ID, FileKey: "k"}
	if err := db.Create(attachment).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	svc := newAttachmentDeleteService()
	_, err := svc.Delete(postActorContext(author), attachment.ID, true)
	if err == nil || xerr.GetHTTPStatus(err) != xerr.StatusForbidden {
		t.Fatalf("Delete(force) error = %v, want 403", err)
	}
}

func TestDeleteAttachmentAuthorDeletesOwnUnreferenced(t *testing.T) {
	db := setupPostAttachmentDB(t)
	author := createTransferUser(t, db, "author", consts.UserRoleAuthor)
	attachment := &entity.Attachment{Name: "img", Path: "upload/a.png", UserID: author.ID, FileKey: "k"}
	if err := db.Create(attachment).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	svc := newAttachmentDeleteService()
	deleted, err := svc.Delete(postActorContext(author), attachment.ID, false)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleted.ID != attachment.ID {
		t.Fatalf("deleted id = %d, want %d", deleted.ID, attachment.ID)
	}
}

func TestDeleteAttachmentBatchAuthorCannotForce(t *testing.T) {
	db := setupPostAttachmentDB(t)
	author := createTransferUser(t, db, "author", consts.UserRoleAuthor)
	attachment := &entity.Attachment{Name: "img", Path: "upload/a.png", UserID: author.ID, FileKey: "k"}
	if err := db.Create(attachment).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	svc := newAttachmentDeleteService()
	if _, err := svc.DeleteBatch(postActorContext(author), []int32{attachment.ID}, true); err == nil ||
		xerr.GetHTTPStatus(err) != xerr.StatusForbidden {
		t.Fatalf("DeleteBatch(force) error = %v, want 403", err)
	}
}