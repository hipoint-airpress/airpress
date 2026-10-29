package impl

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/service"
)

var recoveryCodeDSNSeq int64

func setupRecoveryCodeService(t *testing.T) (service.RecoveryCodeService, context.Context) {
	t.Helper()

	dsn := fmt.Sprintf("file:airpress_recovery_code_%d?mode=memory&cache=shared",
		atomic.AddInt64(&recoveryCodeDSNSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&entity.MfaRecoveryCode{}); err != nil {
		t.Fatalf("auto migrate recovery code: %v", err)
	}
	prev := *dal.Q
	t.Cleanup(func() {
		*dal.Q = prev
		_ = sqlDB.Close()
	})
	dal.SetDefault(db)

	return NewRecoveryCodeService(), context.Background()
}

func countRecoveryRows(t *testing.T) int64 {
	t.Helper()
	rows, err := dal.GetQueryByCtx(context.Background()).MfaRecoveryCode.
		WithContext(context.Background()).Find()
	if err != nil {
		t.Fatalf("find recovery codes: %v", err)
	}
	return int64(len(rows))
}

func TestRecoveryCodeGenerateReturnsUniquePlaintextOnce(t *testing.T) {
	svc, ctx := setupRecoveryCodeService(t)
	const userID int32 = 1

	codes, err := svc.Generate(ctx, userID)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(codes) != 10 {
		t.Fatalf("Generate() returned %d codes, want 10", len(codes))
	}
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if len(code) != 14 || strings.Count(code, "-") != 2 {
			t.Fatalf("code %q is not formatted as XXXX-XXXX-XXXX", code)
		}
		if _, ok := seen[code]; ok {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = struct{}{}
	}
	if got := countRecoveryRows(t); got != 10 {
		t.Fatalf("stored rows = %d, want 10", got)
	}

	rows, err := dal.GetQueryByCtx(ctx).MfaRecoveryCode.WithContext(ctx).Find()
	if err != nil {
		t.Fatalf("find rows: %v", err)
	}
	for _, row := range rows {
		for _, code := range codes {
			if row.CodeHash == code {
				t.Fatal("database must not store plaintext recovery code")
			}
		}
	}
}

func TestRecoveryCodeValidateConsumesOnlyOnce(t *testing.T) {
	svc, ctx := setupRecoveryCodeService(t)
	const userID int32 = 2

	codes, err := svc.Generate(ctx, userID)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// 允许大小写与分隔符不敏感。
	input := strings.ToLower(strings.ReplaceAll(codes[0], "-", " "))
	ok, err := svc.ValidateAndConsume(ctx, userID, input)
	if err != nil {
		t.Fatalf("ValidateAndConsume() error = %v", err)
	}
	if !ok {
		t.Fatal("valid code should be consumed")
	}

	ok, err = svc.ValidateAndConsume(ctx, userID, codes[0])
	if err != nil {
		t.Fatalf("second ValidateAndConsume() error = %v", err)
	}
	if ok {
		t.Fatal("used code must not be accepted twice")
	}

	remaining, err := svc.CountRemaining(ctx, userID)
	if err != nil {
		t.Fatalf("CountRemaining() error = %v", err)
	}
	if remaining != 9 {
		t.Fatalf("remaining = %d, want 9", remaining)
	}
}

func TestRecoveryCodeRejectsWrongCode(t *testing.T) {
	svc, ctx := setupRecoveryCodeService(t)
	const userID int32 = 3

	if _, err := svc.Generate(ctx, userID); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	ok, err := svc.ValidateAndConsume(ctx, userID, "AAAA-BBBB-CCCC")
	if err != nil {
		t.Fatalf("ValidateAndConsume() error = %v", err)
	}
	if ok {
		t.Fatal("wrong code must be rejected")
	}
}

func TestRecoveryCodeRegenerateInvalidatesOldCodes(t *testing.T) {
	svc, ctx := setupRecoveryCodeService(t)
	const userID int32 = 4

	oldCodes, err := svc.Generate(ctx, userID)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	newCodes, err := svc.Generate(ctx, userID)
	if err != nil {
		t.Fatalf("second Generate() error = %v", err)
	}

	ok, err := svc.ValidateAndConsume(ctx, userID, oldCodes[0])
	if err != nil {
		t.Fatalf("ValidateAndConsume(old) error = %v", err)
	}
	if ok {
		t.Fatal("old code must be invalid after regeneration")
	}

	ok, err = svc.ValidateAndConsume(ctx, userID, newCodes[0])
	if err != nil {
		t.Fatalf("ValidateAndConsume(new) error = %v", err)
	}
	if !ok {
		t.Fatal("new code should be accepted")
	}

	if got := countRecoveryRows(t); got != 10 {
		t.Fatalf("stored rows = %d, want 10", got)
	}
}

func TestRecoveryCodeDeleteAll(t *testing.T) {
	svc, ctx := setupRecoveryCodeService(t)
	const userID int32 = 5

	if _, err := svc.Generate(ctx, userID); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if err := svc.DeleteAll(ctx, userID); err != nil {
		t.Fatalf("DeleteAll() error = %v", err)
	}
	remaining, err := svc.CountRemaining(ctx, userID)
	if err != nil {
		t.Fatalf("CountRemaining() error = %v", err)
	}
	if remaining != 0 {
		t.Fatalf("remaining = %d, want 0", remaining)
	}
}
