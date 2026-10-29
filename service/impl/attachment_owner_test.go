package impl

import (
	"context"
	"testing"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/entity"
)

func TestAttachmentConvertToDTOsIncludesOwner(t *testing.T) {
	db := setupPostAttachmentDB(t)
	owner := createTransferUser(t, db, "owner", consts.UserRoleAuthor)
	attachment := &entity.Attachment{
		Name: "a", Path: "upload/a.png", UserID: owner.ID, Type: consts.AttachmentTypeLocal,
	}
	if err := db.Create(attachment).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	svc := &attachmentServiceImpl{
		UserService: &userServiceImpl{},
		FileStorageComposite: fakeFileStorageComposite{
			consts.AttachmentTypeLocal: &fakeFileStorage{
				attachmentType: consts.AttachmentTypeLocal,
				baseURL:        "https://cdn.test",
			},
		},
	}

	dtos, err := svc.ConvertToDTOs(context.Background(), []*entity.Attachment{attachment})
	if err != nil {
		t.Fatalf("ConvertToDTOs() error = %v", err)
	}
	if len(dtos) != 1 {
		t.Fatalf("len(dtos) = %d, want 1", len(dtos))
	}
	if dtos[0].Owner == nil {
		t.Fatal("Owner should not be nil")
	}
	if dtos[0].Owner.ID != owner.ID || dtos[0].Owner.Username != owner.Username {
		t.Fatalf("Owner = %+v, want id %d username %q", dtos[0].Owner, owner.ID, owner.Username)
	}

	single, err := svc.ConvertToDTO(context.Background(), attachment)
	if err != nil {
		t.Fatalf("ConvertToDTO() error = %v", err)
	}
	if single.Owner == nil || single.Owner.Username != owner.Username {
		t.Fatalf("single Owner = %+v", single.Owner)
	}
}