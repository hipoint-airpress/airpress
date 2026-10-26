package impl

import (
	"context"
	"crypto/rand"
	"math/big"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/service"
)

const (
	recoveryCodeCount    = 10
	recoveryCodeLength   = 12
	recoveryCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

type recoveryCodeServiceImpl struct{}

func NewRecoveryCodeService() service.RecoveryCodeService {
	return &recoveryCodeServiceImpl{}
}

func (s *recoveryCodeServiceImpl) Generate(ctx context.Context, userID int32) ([]string, error) {
	if err := s.DeleteAll(ctx, userID); err != nil {
		return nil, err
	}

	codes := make([]string, 0, recoveryCodeCount)
	rows := make([]*entity.MfaRecoveryCode, 0, recoveryCodeCount)
	for i := 0; i < recoveryCodeCount; i++ {
		code, err := generateRecoveryCode()
		if err != nil {
			return nil, err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(normalizeRecoveryCode(code)), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		codes = append(codes, code)
		rows = append(rows, &entity.MfaRecoveryCode{
			UserID:   userID,
			CodeHash: string(hash),
		})
	}

	codeDAL := dal.GetQueryByCtx(ctx).MfaRecoveryCode
	if err := codeDAL.WithContext(ctx).Create(rows...); err != nil {
		return nil, WrapDBErr(err)
	}
	return codes, nil
}

func (s *recoveryCodeServiceImpl) ValidateAndConsume(ctx context.Context, userID int32, code string) (bool, error) {
	normalized := normalizeRecoveryCode(code)
	if normalized == "" {
		return false, nil
	}

	codeDAL := dal.GetQueryByCtx(ctx).MfaRecoveryCode
	rows, err := codeDAL.WithContext(ctx).
		Where(codeDAL.UserID.Eq(userID), codeDAL.UsedTime.IsNull()).
		Find()
	if err != nil {
		return false, WrapDBErr(err)
	}

	for _, row := range rows {
		if bcrypt.CompareHashAndPassword([]byte(row.CodeHash), []byte(normalized)) != nil {
			continue
		}
		now := time.Now()
		result, err := codeDAL.WithContext(ctx).
			Where(codeDAL.ID.Eq(row.ID), codeDAL.UsedTime.IsNull()).
			UpdateSimple(codeDAL.UsedTime.Value(now))
		if err != nil {
			return false, WrapDBErr(err)
		}
		return result.RowsAffected == 1, nil
	}
	return false, nil
}

func (s *recoveryCodeServiceImpl) CountRemaining(ctx context.Context, userID int32) (int64, error) {
	codeDAL := dal.GetQueryByCtx(ctx).MfaRecoveryCode
	count, err := codeDAL.WithContext(ctx).
		Where(codeDAL.UserID.Eq(userID), codeDAL.UsedTime.IsNull()).
		Count()
	if err != nil {
		return 0, WrapDBErr(err)
	}
	return count, nil
}

func (s *recoveryCodeServiceImpl) DeleteAll(ctx context.Context, userID int32) error {
	codeDAL := dal.GetQueryByCtx(ctx).MfaRecoveryCode
	if _, err := codeDAL.WithContext(ctx).Where(codeDAL.UserID.Eq(userID)).Delete(); err != nil {
		return WrapDBErr(err)
	}
	return nil
}

func generateRecoveryCode() (string, error) {
	var builder strings.Builder
	builder.Grow(recoveryCodeLength + 2)
	for i := 0; i < recoveryCodeLength; i++ {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(recoveryCodeAlphabet))))
		if err != nil {
			return "", err
		}
		if i > 0 && i%4 == 0 {
			builder.WriteByte('-')
		}
		builder.WriteByte(recoveryCodeAlphabet[index.Int64()])
	}
	return builder.String(), nil
}

func normalizeRecoveryCode(code string) string {
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return strings.ToUpper(strings.TrimSpace(code))
}
