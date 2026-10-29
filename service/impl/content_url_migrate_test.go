package impl

import (
	"context"
	"fmt"
	"testing"

	"github.com/hipoint-airpress/airpress/model/entity"
)

func TestMigrateContentURLs(t *testing.T) {
	db := setupPostAttachmentDB(t)
	if err := db.AutoMigrate(&entity.Journal{}); err != nil {
		t.Fatalf("auto migrate journal: %v", err)
	}
	if err := db.Create(&entity.Attachment{
		Name: "a", Path: "upload/img/a.png", FileKey: "upload/img/a.png",
	}).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	post := &entity.Post{
		Title:           "post",
		Slug:            "post",
		OriginalContent: "![a](https://old.example.com/upload/img/a.png)",
		FormatContent:   `<img src="https://old.example.com/upload/img/a.png">`,
	}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}
	journal := &entity.Journal{
		SourceContent: "![a](https://old.example.com/upload/img/a.png)",
		Content:       `<img src="https://old.example.com/upload/img/a.png">`,
	}
	if err := db.Create(journal).Error; err != nil {
		t.Fatalf("create journal: %v", err)
	}

	updated, err := MigrateContentURLs(context.Background())
	if err != nil {
		t.Fatalf("MigrateContentURLs() error = %v", err)
	}
	if updated != 2 {
		t.Fatalf("updated = %d, want 2", updated)
	}

	var gotPost entity.Post
	if err := db.First(&gotPost, post.ID).Error; err != nil {
		t.Fatalf("load post: %v", err)
	}
	if gotPost.OriginalContent != "![a](upload/img/a.png)" {
		t.Fatalf("post OriginalContent = %q", gotPost.OriginalContent)
	}
	if gotPost.FormatContent != `<img src="upload/img/a.png">` {
		t.Fatalf("post FormatContent = %q", gotPost.FormatContent)
	}

	var gotJournal entity.Journal
	if err := db.First(&gotJournal, journal.ID).Error; err != nil {
		t.Fatalf("load journal: %v", err)
	}
	if gotJournal.Content != `<img src="upload/img/a.png">` {
		t.Fatalf("journal Content = %q", gotJournal.Content)
	}
	if gotJournal.SourceContent != "![a](upload/img/a.png)" {
		t.Fatalf("journal SourceContent = %q", gotJournal.SourceContent)
	}

	again, err := MigrateContentURLs(context.Background())
	if err != nil {
		t.Fatalf("second MigrateContentURLs() error = %v", err)
	}
	if again != 0 {
		t.Fatalf("second run updated = %d, want 0", again)
	}
}

func TestMigrateContentURLsPaginates(t *testing.T) {
	db := setupPostAttachmentDB(t)
	if err := db.AutoMigrate(&entity.Journal{}); err != nil {
		t.Fatalf("auto migrate journal: %v", err)
	}
	if err := db.Create(&entity.Attachment{
		Name: "a", Path: "upload/img/a.png", FileKey: "upload/img/a.png",
	}).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	posts := make([]entity.Post, 205)
	for i := range posts {
		posts[i] = entity.Post{
			Title:           fmt.Sprintf("post-%d", i),
			Slug:            fmt.Sprintf("post-%d", i),
			OriginalContent: "![a](https://old.example.com/upload/img/a.png)",
			FormatContent:   `<img src="https://old.example.com/upload/img/a.png">`,
		}
	}
	if err := db.Create(&posts).Error; err != nil {
		t.Fatalf("create posts: %v", err)
	}

	updated, err := MigrateContentURLs(context.Background())
	if err != nil {
		t.Fatalf("MigrateContentURLs() error = %v", err)
	}
	if updated != 205 {
		t.Fatalf("updated = %d, want 205", updated)
	}

	for _, index := range []int{0, 199, 204} {
		var got entity.Post
		if err := db.First(&got, posts[index].ID).Error; err != nil {
			t.Fatalf("load post %d: %v", index, err)
		}
		if got.OriginalContent != "![a](upload/img/a.png)" {
			t.Fatalf("post %d OriginalContent = %q", index, got.OriginalContent)
		}
		if got.FormatContent != `<img src="upload/img/a.png">` {
			t.Fatalf("post %d FormatContent = %q", index, got.FormatContent)
		}
	}

	again, err := MigrateContentURLs(context.Background())
	if err != nil {
		t.Fatalf("second MigrateContentURLs() error = %v", err)
	}
	if again != 0 {
		t.Fatalf("second run updated = %d, want 0", again)
	}
}
