package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestCreativeRepository_ListByDevice_FiltersActiveCampaigns(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewCreativeRepository(db)

	mock.ExpectQuery("JOIN campaigns ca ON ca\\.id = cr\\.campaign_id").
		WithArgs("kcmo_web", 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "url", "file_path", "size", "campaign_id", "selected_days", "time_slots", "devices", "uploaded_at"}))

	_, err = repo.ListByDevice(context.Background(), "kcmo_web", false, time.Now().UTC(), 10, 0)
	if err != nil {
		t.Fatalf("ListByDevice: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestCreativeRepository_CountByDevice_FiltersActiveCampaigns(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewCreativeRepository(db)

	mock.ExpectQuery("JOIN campaigns ca ON ca\\.id = cr\\.campaign_id").
		WithArgs("kcmo_web").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	_, err = repo.CountByDevice(context.Background(), "kcmo_web", false, time.Now().UTC())
	if err != nil {
		t.Fatalf("CountByDevice: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
