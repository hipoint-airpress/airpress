package assembler

import (
	"context"
	"testing"

	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/service"
)

// countingJournalService 统计 ConvertToDTO 调用次数，用于验证同一 journal 只转换一次。
type countingJournalService struct {
	service.JournalService
	journals map[int32]*entity.Journal
	calls    int
}

func (s *countingJournalService) GetByJournalIDs(context.Context, []int32) (map[int32]*entity.Journal, error) {
	return s.journals, nil
}

func (s *countingJournalService) ConvertToDTO(_ context.Context, journal *entity.Journal) (*dto.Journal, error) {
	s.calls++
	return &dto.Journal{ID: journal.ID, Content: journal.Content}, nil
}

type stubCommentAssembler struct{ BaseCommentAssembler }

func (stubCommentAssembler) ConvertToDTO(_ context.Context, comment *entity.Comment) (*dto.Comment, error) {
	return &dto.Comment{ID: comment.ID}, nil
}

func TestJournalCommentAssemblerConvertToWithJournalCachesJournalDTO(t *testing.T) {
	journalService := &countingJournalService{
		journals: map[int32]*entity.Journal{
			1: {ID: 1, Content: "content"},
		},
	}
	assembler := &journalCommentAssembler{
		BaseCommentAssembler: stubCommentAssembler{},
		JournalService:       journalService,
	}
	comments := []*entity.Comment{
		{ID: 10, PostID: 1},
		{ID: 11, PostID: 1},
	}

	got, err := assembler.ConvertToWithJournal(context.Background(), comments)
	if err != nil {
		t.Fatalf("ConvertToWithJournal() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(result) = %d, want 2", len(got))
	}
	if journalService.calls != 1 {
		t.Fatalf("ConvertToDTO calls = %d, want 1 (same journal should be resolved once)", journalService.calls)
	}
	for i, item := range got {
		if item.Journal == nil || item.Journal.ID != 1 {
			t.Fatalf("result[%d].Journal = %#v, want journal 1", i, item.Journal)
		}
	}
}
