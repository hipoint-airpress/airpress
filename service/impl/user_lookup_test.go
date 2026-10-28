package impl

import (
	"context"
	"testing"

	"github.com/hipoint-airpress/airpress/consts"
)

func TestUserServiceGetByIDs(t *testing.T) {
	f := setupUserManagement(t)
	first := f.createUser(t, "first", consts.UserRoleAuthor)
	second := f.createUser(t, "second", consts.UserRoleEditor)

	svc := &userServiceImpl{}
	users, err := svc.GetByIDs(context.Background(), []int32{first.ID, second.ID, 9999})
	if err != nil {
		t.Fatalf("GetByIDs() error = %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("users = %d, want 2", len(users))
	}
	seen := map[int32]string{users[0].ID: users[0].Username, users[1].ID: users[1].Username}
	if seen[first.ID] != "first" || seen[second.ID] != "second" {
		t.Fatalf("unexpected users: %+v", users)
	}

	empty, err := svc.GetByIDs(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetByIDs(nil) error = %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("GetByIDs(nil) = %d users, want 0", len(empty))
	}
}
