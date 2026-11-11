package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/domain"
)

// routerAs mounts the item, sign-out and file routes with nil services, so
// only requests rejected before reaching a service can be asserted.
func routerAs(perms ...string) http.Handler {
	set := map[string]struct{}{}
	for _, p := range perms {
		set[p] = struct{}{}
	}
	u := &auth.User{ID: uuid.New(), Permissions: set}
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(auth.WithUser(req.Context(), u)))
		})
	})
	r.Route("/items", (&itemHandler{}).routes)
	r.Route("/signouts", (&signoutHandler{}).routes)
	r.Route("/files", (&fileHandler{}).routes)
	return r
}

func TestRoutesRejectMissingPermissions(t *testing.T) {
	id := uuid.NewString()
	readonly := routerAs(domain.PermItemView, domain.PermSignoutView, domain.PermAuditView)
	frontdesk := routerAs(domain.PermLostfoundView, domain.PermLostfoundCreate, domain.PermFileUpload)

	cases := []struct {
		name   string
		h      http.Handler
		method string
		path   string
	}{
		{"readonly create item", readonly, http.MethodPost, "/items"},
		{"readonly edit item", readonly, http.MethodPatch, "/items/" + id},
		{"readonly retire item", readonly, http.MethodPost, "/items/" + id + "/retire"},
		{"readonly move item", readonly, http.MethodPost, "/items/" + id + "/move"},
		{"readonly peek barcode", readonly, http.MethodGet, "/items/barcodes/peek-next"},
		{"readonly create signout", readonly, http.MethodPost, "/signouts"},
		{"readonly return signout", readonly, http.MethodPost, "/signouts/" + id + "/return"},
		{"readonly upload", readonly, http.MethodPost, "/files/upload"},
		{"readonly delete file", readonly, http.MethodDelete, "/files/" + id},
		{"readonly list lostfound files", readonly, http.MethodGet, "/files?entity_type=lostfound&entity_id=" + id},
		{"frontdesk list items", frontdesk, http.MethodGet, "/items"},
		{"frontdesk view item", frontdesk, http.MethodGet, "/items/" + id},
		{"frontdesk list signouts", frontdesk, http.MethodGet, "/signouts"},
		{"frontdesk list item files", frontdesk, http.MethodGet, "/files?entity_type=item&entity_id=" + id},
		{"frontdesk presign item upload", frontdesk, http.MethodPost, "/files/presign-upload"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var body *strings.Reader
			if c.name == "frontdesk presign item upload" {
				body = strings.NewReader(`{"entity_type":"item","entity_id":"` + id + `","file_type":"image","original_filename":"a.png"}`)
			} else {
				body = strings.NewReader("{}")
			}
			req := httptest.NewRequest(c.method, c.path, body)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			c.h.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s %s: status %d, want 403 (%s)", c.method, c.path, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRoleCatalogPublishesGlobalPrefixedRoles(t *testing.T) {
	rec := httptest.NewRecorder()
	roleCatalogHandler(auth.DefaultAccessConfig())(rec, httptest.NewRequest(http.MethodGet, "/api/role-catalog", nil))
	var body struct {
		Revision string             `json:"revision"`
		Roles    []roleCatalogEntry `json:"roles"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Revision == "" || rec.Header().Get("ETag") != `"`+body.Revision+`"` {
		t.Fatalf("revision %q, ETag %q", body.Revision, rec.Header().Get("ETag"))
	}
	seen := map[string]roleCatalogEntry{}
	for _, r := range body.Roles {
		if !strings.HasPrefix(r.Key, "stash_") || r.Scope != "organization" || r.Group != r.Key || r.GroupDisplay != r.Key || r.Dynamic {
			t.Errorf("bad catalog entry %+v", r)
		}
		if _, dup := seen[r.Key]; dup {
			t.Errorf("duplicate key %s", r.Key)
		}
		seen[r.Key] = r
	}
	for key, privileged := range map[string]bool{"stash_admin": true, "stash_inventory_manager": true, "stash_volunteer": false, "stash_frontdesk": false, "stash_readonly": false} {
		r, ok := seen[key]
		if !ok || r.Privileged != privileged {
			t.Errorf("%s: present %v, privileged %v, want %v", key, ok, r.Privileged, privileged)
		}
	}
}

func TestFilesRejectUnknownEntityType(t *testing.T) {
	h := routerAs(domain.PermItemView)
	req := httptest.NewRequest(http.MethodGet, "/files?entity_type=user&entity_id="+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}
