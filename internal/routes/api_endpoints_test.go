package routes

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"scm/internal/config"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

type endpointCase struct {
	name       string
	method     string
	path       string
	protected  bool
	wantStatus int // 0 means "just ensure routed (not chi default 404)"
}

func TestAPIv1EndpointsAreWired(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	r, err := SetupRoutes(db, nil, &config.Config{JWTSecret: "dev"}, &config.S3Config{}, &noopCampaignRepo{}, &noopCreativeRepo{}, &noopUserRepo{}, &noopAdvertiserRepo{}, &noopDeviceRepo{}, &noopProjectRepo{}, &noopRoleRepo{}, &noopPermissionRepo{}, &noopUserRoleRepo{}, &noopPlaceExchangeTokenRepo{}, &noopLegacyRevisionRepo{})
	if err != nil {
		t.Fatalf("SetupRoutes: %v", err)
	}

	defaultNotFoundBody := "404 page not found\n"

	ensureRouted := func(t *testing.T, w *httptest.ResponseRecorder, tc endpointCase) {
		t.Helper()
		if w.Code == http.StatusNotFound && w.Body.String() == defaultNotFoundBody {
			t.Fatalf("endpoint appears unregistered (chi default 404): %s %s", tc.method, tc.path)
		}
	}

	cases := []endpointCase{
		// Public
		{name: "debug env", method: http.MethodGet, path: "/api/v1/debug/env"},
		{name: "auth signup", method: http.MethodPost, path: "/api/v1/auth/signup"},
		{name: "auth login", method: http.MethodPost, path: "/api/v1/auth/login"},
		{name: "auth forgot-password", method: http.MethodPost, path: "/api/v1/auth/forgot-password"},
		{name: "auth reset-password", method: http.MethodPost, path: "/api/v1/auth/reset-password"},
		{name: "public creatives by device", method: http.MethodGet, path: "/api/v1/creatives/device/kiosk-1"},

		// Protected: RBAC
		{name: "roles list", method: http.MethodGet, path: "/api/v1/roles/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "roles create", method: http.MethodPost, path: "/api/v1/roles/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "roles get", method: http.MethodGet, path: "/api/v1/roles/r1/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "roles update", method: http.MethodPut, path: "/api/v1/roles/r1/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "roles delete", method: http.MethodDelete, path: "/api/v1/roles/r1/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "role permissions get", method: http.MethodGet, path: "/api/v1/roles/r1/permissions", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "role permissions set", method: http.MethodPut, path: "/api/v1/roles/r1/permissions", protected: true, wantStatus: http.StatusUnauthorized},

		{name: "permissions list", method: http.MethodGet, path: "/api/v1/permissions/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "permissions create", method: http.MethodPost, path: "/api/v1/permissions/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "permissions get", method: http.MethodGet, path: "/api/v1/permissions/p1/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "permissions update", method: http.MethodPut, path: "/api/v1/permissions/p1/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "permissions delete", method: http.MethodDelete, path: "/api/v1/permissions/p1/", protected: true, wantStatus: http.StatusUnauthorized},

		{name: "user roles list", method: http.MethodGet, path: "/api/v1/users/u1/roles/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "user roles set", method: http.MethodPut, path: "/api/v1/users/u1/roles/", protected: true, wantStatus: http.StatusUnauthorized},

		// Protected: Campaigns
		{name: "campaigns search", method: http.MethodGet, path: "/api/v1/campaigns/search?query=x", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "campaigns list", method: http.MethodGet, path: "/api/v1/campaigns/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "campaigns by advertiser", method: http.MethodGet, path: "/api/v1/campaigns/advertiser/a1", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "campaigns create", method: http.MethodPost, path: "/api/v1/campaigns/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "campaigns get", method: http.MethodGet, path: "/api/v1/campaigns/c1/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "campaigns impressions", method: http.MethodGet, path: "/api/v1/campaigns/c1/impressions", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "campaigns update", method: http.MethodPut, path: "/api/v1/campaigns/c1/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "campaigns delete", method: http.MethodDelete, path: "/api/v1/campaigns/c1/", protected: true, wantStatus: http.StatusUnauthorized},

		// Protected: Advertisers
		{name: "advertisers search", method: http.MethodGet, path: "/api/v1/advertisers/search?query=x", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "advertisers list", method: http.MethodGet, path: "/api/v1/advertisers/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "advertisers create", method: http.MethodPost, path: "/api/v1/advertisers/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "advertisers get", method: http.MethodGet, path: "/api/v1/advertisers/a1/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "advertisers update", method: http.MethodPut, path: "/api/v1/advertisers/a1/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "advertisers delete", method: http.MethodDelete, path: "/api/v1/advertisers/a1/", protected: true, wantStatus: http.StatusUnauthorized},

		// Protected: Creatives
		{name: "creatives search", method: http.MethodGet, path: "/api/v1/creatives/search?query=x", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "creatives list", method: http.MethodGet, path: "/api/v1/creatives/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "creatives suggestions", method: http.MethodPost, path: "/api/v1/creatives/suggestions", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "creatives upload", method: http.MethodPost, path: "/api/v1/creatives/upload", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "creatives by campaign", method: http.MethodGet, path: "/api/v1/creatives/campaign/c1", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "creatives get", method: http.MethodGet, path: "/api/v1/creatives/cr1/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "creatives update", method: http.MethodPut, path: "/api/v1/creatives/cr1/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "creatives delete", method: http.MethodDelete, path: "/api/v1/creatives/cr1/", protected: true, wantStatus: http.StatusUnauthorized},

		// Protected: Sync
		{name: "sync console", method: http.MethodPost, path: "/api/v1/sync/console", protected: true, wantStatus: http.StatusUnauthorized},

		// Protected: Replicator
		{name: "replicator ad_posters", method: http.MethodGet, path: "/api/v1/replicator/ad_posters?city=opt", protected: true, wantStatus: http.StatusUnauthorized},

		// Protected: Projects
		{name: "projects search", method: http.MethodGet, path: "/api/v1/projects/search?query=x", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "projects list", method: http.MethodGet, path: "/api/v1/projects/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "projects get", method: http.MethodGet, path: "/api/v1/projects/p1", protected: true, wantStatus: http.StatusUnauthorized},

		// Protected: Devices
		{name: "devices query", method: http.MethodPost, path: "/api/v1/devices/query", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "devices search", method: http.MethodGet, path: "/api/v1/devices/search?query=x", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "devices counts regions", method: http.MethodGet, path: "/api/v1/devices/counts/regions", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "devices list", method: http.MethodGet, path: "/api/v1/devices/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "devices get", method: http.MethodGet, path: "/api/v1/devices/host-1", protected: true, wantStatus: http.StatusUnauthorized},

		// Protected: Venues
		{name: "venues search", method: http.MethodGet, path: "/api/v1/venues/search?query=x", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "venues list", method: http.MethodGet, path: "/api/v1/venues/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "venues create", method: http.MethodPost, path: "/api/v1/venues/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "venues get", method: http.MethodGet, path: "/api/v1/venues/v1", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "venues update", method: http.MethodPut, path: "/api/v1/venues/v1", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "venues delete", method: http.MethodDelete, path: "/api/v1/venues/v1", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "venues add devices", method: http.MethodPost, path: "/api/v1/venues/v1/devices", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "venues remove devices", method: http.MethodDelete, path: "/api/v1/venues/v1/devices", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "venues get devices", method: http.MethodGet, path: "/api/v1/venues/v1/devices", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "venues list by device", method: http.MethodGet, path: "/api/v1/venues/devices/d1/venues", protected: true, wantStatus: http.StatusUnauthorized},

		// Protected: Users
		{name: "users list", method: http.MethodGet, path: "/api/v1/users/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "users get", method: http.MethodGet, path: "/api/v1/users/u1/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "users update", method: http.MethodPut, path: "/api/v1/users/u1/", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "users change password", method: http.MethodPut, path: "/api/v1/users/u1/password", protected: true, wantStatus: http.StatusUnauthorized},
		{name: "users delete", method: http.MethodDelete, path: "/api/v1/users/u1/", protected: true, wantStatus: http.StatusUnauthorized},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var body *bytes.Reader
			switch tc.method {
			case http.MethodPost, http.MethodPut, http.MethodPatch:
				body = bytes.NewReader([]byte(`{}`))
			default:
				body = bytes.NewReader(nil)
			}

			req := httptest.NewRequest(tc.method, tc.path, body)
			if tc.method == http.MethodPost || tc.method == http.MethodPut || tc.method == http.MethodPatch {
				req.Header.Set("Content-Type", "application/json")
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if tc.wantStatus != 0 {
				if w.Code != tc.wantStatus {
					t.Fatalf("expected status %d got %d (body=%q)", tc.wantStatus, w.Code, w.Body.String())
				}
				return
			}

			ensureRouted(t, w, tc)
		})
	}
}
