package impl

import (
	"testing"

	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

// 复用 post_attachment_sync_test.go 中的 setupPostAttachmentDB 与 postActorContext。
func createTransferUser(t *testing.T, db *gorm.DB, username string, role consts.UserRole) *entity.User {
	t.Helper()
	user := &entity.User{
		Username: username,
		Nickname: username,
		Email:    username + "@example.com",
		Password: "hashed",
		Role:     role,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func TestTransferAuthorMovesPostAndAttachments(t *testing.T) {
	db := setupPostAttachmentDB(t)
	editor := createTransferUser(t, db, "editor", consts.UserRoleEditor)
	source := createTransferUser(t, db, "source", consts.UserRoleAuthor)
	target := createTransferUser(t, db, "target", consts.UserRoleAuthor)

	attachment := &entity.Attachment{Name: "img", Path: "upload/image/2026/10/a.png", UserID: source.ID}
	if err := db.Create(attachment).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	post := &entity.Post{Title: "post", Slug: "post", Type: consts.PostTypePost, AuthorID: source.ID}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}
	relation := &entity.PostAttachment{PostID: post.ID, AttachmentID: attachment.ID}
	if err := db.Create(relation).Error; err != nil {
		t.Fatalf("create relation: %v", err)
	}

	result, err := (postServiceImpl{}).TransferAuthor(postActorContext(editor), []int32{post.ID}, target.ID)
	if err != nil {
		t.Fatalf("TransferAuthor() error = %v", err)
	}
	if result.PostCount != 1 || result.AttachmentCount != 1 {
		t.Fatalf("result = %+v, want post 1 attachment 1", result)
	}

	var storedPost entity.Post
	if err := db.First(&storedPost, post.ID).Error; err != nil {
		t.Fatalf("load post: %v", err)
	}
	if storedPost.AuthorID != target.ID {
		t.Fatalf("post author = %d, want %d", storedPost.AuthorID, target.ID)
	}
	var storedAttachment entity.Attachment
	if err := db.First(&storedAttachment, attachment.ID).Error; err != nil {
		t.Fatalf("load attachment: %v", err)
	}
	if storedAttachment.UserID != target.ID {
		t.Fatalf("attachment user = %d, want %d", storedAttachment.UserID, target.ID)
	}
}

func TestTransferAuthorSkipsSheets(t *testing.T) {
	db := setupPostAttachmentDB(t)
	editor := createTransferUser(t, db, "editor", consts.UserRoleEditor)
	source := createTransferUser(t, db, "source", consts.UserRoleAuthor)
	target := createTransferUser(t, db, "target", consts.UserRoleAuthor)

	sheet := &entity.Post{Title: "sheet", Slug: "sheet", Type: consts.PostTypeSheet, AuthorID: source.ID}
	if err := db.Create(sheet).Error; err != nil {
		t.Fatalf("create sheet: %v", err)
	}

	_, err := (postServiceImpl{}).TransferAuthor(postActorContext(editor), []int32{sheet.ID}, target.ID)
	if err == nil {
		t.Fatal("transferring only sheets should fail")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusBadRequest {
		t.Fatalf("status = %d, want %d", got, xerr.StatusBadRequest)
	}
}

func TestTransferAuthorRejectsAuthorRole(t *testing.T) {
	db := setupPostAttachmentDB(t)
	source := createTransferUser(t, db, "source", consts.UserRoleAuthor)
	target := createTransferUser(t, db, "target", consts.UserRoleAuthor)

	_, err := (postServiceImpl{}).TransferAuthor(postActorContext(source), []int32{1}, target.ID)
	if err == nil {
		t.Fatal("author role should not transfer posts")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusForbidden {
		t.Fatalf("status = %d, want %d", got, xerr.StatusForbidden)
	}
}

func TestTransferAuthorRejectsDisabledTarget(t *testing.T) {
	db := setupPostAttachmentDB(t)
	editor := createTransferUser(t, db, "editor", consts.UserRoleEditor)
	source := createTransferUser(t, db, "source", consts.UserRoleAuthor)
	target := createTransferUser(t, db, "target", consts.UserRoleAuthor)
	if err := db.Model(target).Update("disabled", true).Error; err != nil {
		t.Fatalf("disable target: %v", err)
	}
	post := &entity.Post{Title: "post", Slug: "post", Type: consts.PostTypePost, AuthorID: source.ID}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}

	_, err := (postServiceImpl{}).TransferAuthor(postActorContext(editor), []int32{post.ID}, target.ID)
	if err == nil {
		t.Fatal("disabled target should be rejected")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusBadRequest {
		t.Fatalf("status = %d, want %d", got, xerr.StatusBadRequest)
	}
}

func TestTransferAuthorKeepsThirdPartyAttachments(t *testing.T) {
	db := setupPostAttachmentDB(t)
	editor := createTransferUser(t, db, "editor", consts.UserRoleEditor)
	source := createTransferUser(t, db, "source", consts.UserRoleAuthor)
	third := createTransferUser(t, db, "third", consts.UserRoleAuthor)
	target := createTransferUser(t, db, "target", consts.UserRoleAuthor)

	attachment := &entity.Attachment{Name: "img", Path: "upload/image/2026/10/a.png", UserID: third.ID}
	if err := db.Create(attachment).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	post := &entity.Post{Title: "post", Slug: "post", Type: consts.PostTypePost, AuthorID: source.ID}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}
	relation := &entity.PostAttachment{PostID: post.ID, AttachmentID: attachment.ID}
	if err := db.Create(relation).Error; err != nil {
		t.Fatalf("create relation: %v", err)
	}

	result, err := (postServiceImpl{}).TransferAuthor(postActorContext(editor), []int32{post.ID}, target.ID)
	if err != nil {
		t.Fatalf("TransferAuthor() error = %v", err)
	}
	if result.PostCount != 1 || result.AttachmentCount != 0 {
		t.Fatalf("result = %+v, want post 1 attachment 0", result)
	}

	var storedPost entity.Post
	if err := db.First(&storedPost, post.ID).Error; err != nil {
		t.Fatalf("load post: %v", err)
	}
	if storedPost.AuthorID != target.ID {
		t.Fatalf("post author = %d, want %d", storedPost.AuthorID, target.ID)
	}
	var storedAttachment entity.Attachment
	if err := db.First(&storedAttachment, attachment.ID).Error; err != nil {
		t.Fatalf("load attachment: %v", err)
	}
	if storedAttachment.UserID != third.ID {
		t.Fatalf("attachment user = %d, want %d", storedAttachment.UserID, third.ID)
	}
}

func TestTransferAuthorDeduplicatesSharedAttachment(t *testing.T) {
	db := setupPostAttachmentDB(t)
	editor := createTransferUser(t, db, "editor", consts.UserRoleEditor)
	sourceA := createTransferUser(t, db, "source-a", consts.UserRoleAuthor)
	sourceB := createTransferUser(t, db, "source-b", consts.UserRoleAuthor)
	target := createTransferUser(t, db, "target", consts.UserRoleAuthor)

	shared := &entity.Attachment{Name: "shared", Path: "upload/image/2026/10/shared.png", UserID: sourceA.ID}
	if err := db.Create(shared).Error; err != nil {
		t.Fatalf("create shared attachment: %v", err)
	}
	postA := &entity.Post{Title: "post-a", Slug: "post-a", Type: consts.PostTypePost, AuthorID: sourceA.ID}
	postB := &entity.Post{Title: "post-b", Slug: "post-b", Type: consts.PostTypePost, AuthorID: sourceB.ID}
	if err := db.Create(postA).Error; err != nil {
		t.Fatalf("create post a: %v", err)
	}
	if err := db.Create(postB).Error; err != nil {
		t.Fatalf("create post b: %v", err)
	}
	for _, post := range []*entity.Post{postA, postB} {
		relation := &entity.PostAttachment{PostID: post.ID, AttachmentID: shared.ID}
		if err := db.Create(relation).Error; err != nil {
			t.Fatalf("create relation for post %d: %v", post.ID, err)
		}
	}

	result, err := (postServiceImpl{}).TransferAuthor(postActorContext(editor),
		[]int32{postA.ID, postB.ID}, target.ID)
	if err != nil {
		t.Fatalf("TransferAuthor() error = %v", err)
	}
	if result.PostCount != 2 || result.AttachmentCount != 1 {
		t.Fatalf("result = %+v, want post 2 attachment 1", result)
	}

	for _, post := range []*entity.Post{postA, postB} {
		var storedPost entity.Post
		if err := db.First(&storedPost, post.ID).Error; err != nil {
			t.Fatalf("load post %d: %v", post.ID, err)
		}
		if storedPost.AuthorID != target.ID {
			t.Fatalf("post %d author = %d, want %d", post.ID, storedPost.AuthorID, target.ID)
		}
	}
	var storedAttachment entity.Attachment
	if err := db.First(&storedAttachment, shared.ID).Error; err != nil {
		t.Fatalf("load shared attachment: %v", err)
	}
	if storedAttachment.UserID != target.ID {
		t.Fatalf("shared attachment user = %d, want %d", storedAttachment.UserID, target.ID)
	}
}

func TestTransferAuthorTransfersPostsButSkipsSheets(t *testing.T) {
	db := setupPostAttachmentDB(t)
	editor := createTransferUser(t, db, "editor", consts.UserRoleEditor)
	source := createTransferUser(t, db, "source", consts.UserRoleAuthor)
	target := createTransferUser(t, db, "target", consts.UserRoleAuthor)

	sheet := &entity.Post{Title: "sheet", Slug: "sheet", Type: consts.PostTypeSheet, AuthorID: source.ID}
	post := &entity.Post{Title: "post", Slug: "post", Type: consts.PostTypePost, AuthorID: source.ID}
	if err := db.Create(sheet).Error; err != nil {
		t.Fatalf("create sheet: %v", err)
	}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}

	result, err := (postServiceImpl{}).TransferAuthor(postActorContext(editor),
		[]int32{sheet.ID, post.ID}, target.ID)
	if err != nil {
		t.Fatalf("TransferAuthor() error = %v", err)
	}
	if result.PostCount != 1 || result.AttachmentCount != 0 {
		t.Fatalf("result = %+v, want post 1 attachment 0", result)
	}

	var storedSheet entity.Post
	if err := db.First(&storedSheet, sheet.ID).Error; err != nil {
		t.Fatalf("load sheet: %v", err)
	}
	if storedSheet.AuthorID != source.ID {
		t.Fatalf("sheet author = %d, want unchanged %d", storedSheet.AuthorID, source.ID)
	}
	var storedPost entity.Post
	if err := db.First(&storedPost, post.ID).Error; err != nil {
		t.Fatalf("load post: %v", err)
	}
	if storedPost.AuthorID != target.ID {
		t.Fatalf("post author = %d, want %d", storedPost.AuthorID, target.ID)
	}
}
