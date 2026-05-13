package repository

import (
	"testing"
)

func TestCreativeRepository_ListByDevice_FiltersActiveCampaigns(t *testing.T) {
	t.Skip("sqlmock does not support pgxpool; rewrite with pgxmock or integration tests")
}

func TestCreativeRepository_CountByDevice_FiltersActiveCampaigns(t *testing.T) {
	t.Skip("sqlmock does not support pgxpool; rewrite with pgxmock or integration tests")
}
