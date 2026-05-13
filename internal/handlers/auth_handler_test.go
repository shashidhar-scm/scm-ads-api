package handlers

import (
	"context"
	"database/sql"
	"testing"

	"scm/internal/models"
)

type noopUserRepo struct{}

func (noopUserRepo) Create(context.Context, *models.User) error { return nil }
func (noopUserRepo) GetByID(context.Context, string) (*models.User, error) {
	return nil, sql.ErrNoRows
}
func (noopUserRepo) GetByEmail(context.Context, string) (*models.User, error) {
	return nil, sql.ErrNoRows
}
func (noopUserRepo) GetByIdentifier(context.Context, string) (*models.User, error) {
	return nil, sql.ErrNoRows
}
func (noopUserRepo) List(context.Context, int, int) ([]models.User, error) {
	return nil, nil
}
func (noopUserRepo) Count(context.Context) (int, error) { return 0, nil }
func (noopUserRepo) ListAll(context.Context) ([]models.User, error) {
	return nil, nil
}
func (noopUserRepo) UpdateProfile(context.Context, string, *models.UpdateUserRequest) error {
	return nil
}
func (noopUserRepo) UpdatePasswordHash(context.Context, string, string) error {
	return nil
}
func (noopUserRepo) Delete(context.Context, string) error { return nil }

func TestGoogleAuthExistingUserSuccess(t *testing.T) {
	t.Skip("sqlmock does not support pgxpool; rewrite with pgxmock or integration tests")
}

func TestGoogleAuthCreatesUserAndAssignsRole(t *testing.T) {
	t.Skip("sqlmock does not support pgxpool; rewrite with pgxmock or integration tests")
}

func TestGoogleAuthInvalidToken(t *testing.T) {
	t.Skip("sqlmock does not support pgxpool; rewrite with pgxmock or integration tests")
}

func TestForgotPasswordReturnsTokenWhenEnabled(t *testing.T) {
	t.Skip("sqlmock does not support pgxpool; rewrite with pgxmock or integration tests")
}

func TestSignupWeakPasswordReturnsJSON(t *testing.T) {
	t.Skip("sqlmock does not support pgxpool; rewrite with pgxmock or integration tests")
}

func TestSignupInvalidUserNameReturnsJSON(t *testing.T) {
	t.Skip("sqlmock does not support pgxpool; rewrite with pgxmock or integration tests")
}

func TestResetPasswordSuccess(t *testing.T) {
	t.Skip("sqlmock does not support pgxpool; rewrite with pgxmock or integration tests")
}

func TestSignupSuccess(t *testing.T) {
	t.Skip("sqlmock does not support pgxpool; rewrite with pgxmock or integration tests")
}

func TestSignupDuplicateEmailReturnsJSON(t *testing.T) {
	t.Skip("sqlmock does not support pgxpool; rewrite with pgxmock or integration tests")
}

func TestLoginSuccess(t *testing.T) {
	t.Skip("sqlmock does not support pgxpool; rewrite with pgxmock or integration tests")
}
