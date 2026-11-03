// Package http builds the chi router and ties handlers to dependencies.
package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"

	"github.com/AGlElectronics/stash/backend/internal/auth"
	"github.com/AGlElectronics/stash/backend/internal/config"
	"github.com/AGlElectronics/stash/backend/internal/domain"
	"github.com/AGlElectronics/stash/backend/internal/files"
	"github.com/AGlElectronics/stash/backend/internal/service"
)

// Deps groups the runtime dependencies the HTTP layer needs.
type Deps struct {
	Config       *config.Config
	Pool         *pgxpool.Pool
	Logger       zerolog.Logger
	Files        *files.Service
	OIDCVerifier *auth.OIDCVerifier
	// Access is the config-file-driven role/permission catalog and OIDC
	// group → role bindings. It is the only source of access control.
	Access *auth.AccessConfig
}

// NewRouter wires the chi router with middleware, CORS and routes.
// Version is exposed via /api/health.
func NewRouter(deps Deps, version string) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(60 * time.Second))

	r.Use(hlog.NewHandler(deps.Logger))
	r.Use(hlog.AccessHandler(func(req *http.Request, status, size int, duration time.Duration) {
		hlog.FromRequest(req).Info().
			Str("method", req.Method).
			Str("path", req.URL.Path).
			Int("status", status).
			Int("size", size).
			Dur("duration", duration).
			Msg("http")
	}))
	r.Use(hlog.RequestIDHandler("request_id", "X-Request-ID"))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   deps.Config.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Dev-User", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	health := newHealthHandler(deps.Pool, version)
	resolver := auth.NewResolver(deps.Pool, deps.Access)
	authMW := auth.Middleware(deps.Config, resolver, deps.OIDCVerifier)

	depts := &departmentHandler{svc: service.NewDepartmentService(deps.Pool)}
	cats := &categoryHandler{svc: service.NewCategoryService(deps.Pool)}
	locs := &locationHandler{svc: service.NewLocationService(deps.Pool)}
	items := &itemHandler{svc: service.NewItemService(deps.Pool), files: deps.Files}
	signouts := &signoutHandler{svc: service.NewSignoutService(deps.Pool)}
	lostfound := &lostFoundHandler{svc: service.NewLostFoundService(deps.Pool)}
	fileH := &fileHandler{svc: deps.Files}
	settings := &settingsHandler{svc: service.NewSettingsService(deps.Pool)}

	publicLF := &publicLostFoundHandler{pool: deps.Pool, files: deps.Files}

	r.Route("/api", func(api chi.Router) {
		api.Get("/health", health.get)
		api.Get("/role-catalog", roleCatalogHandler(deps.Access))

		if deps.Config.PublicLostFoundClaimEnabled {
			api.Route("/public/lostfound", func(pub chi.Router) {
				pub.Get("/", publicLF.search)
				pub.Get("/{reportNumber}/photo", publicLF.photo)
				pub.Get("/{reportNumber}", publicLF.getByNumber)
			})
		}

		api.Group(func(secure chi.Router) {
			secure.Use(authMW)
			secure.Get("/me", meHandler)

			// Reads always require at least the matching view permission anywhere.
			// Writes are gated per-method via small inline groups so the same
			// route can apply different permissions to different verbs.
			secure.Group(func(g chi.Router) {
				g.Use(requireAny(domain.PermItemView))
				g.Get("/departments", depts.list)
				g.Get("/departments/{id}", depts.get)
				g.Get("/categories", cats.list)
				g.Get("/categories/{id}", cats.get)
				g.Get("/locations", locs.list)
				g.Get("/locations/{id}", locs.get)
			})

			secure.Group(func(g chi.Router) {
				g.Use(requireAnyGlobal(
					domain.PermDepartmentManage,
					domain.PermItemCreate,
					domain.PermItemEdit,
				))
				g.Post("/departments", depts.create)
			})
			secure.Group(func(g chi.Router) {
				g.Use(requireGlobal(domain.PermDepartmentManage))
				g.Patch("/departments/{id}", depts.update)
				g.Delete("/departments/{id}", depts.delete)
			})
			secure.Group(func(g chi.Router) {
				g.Use(requireAnyGlobal(
					domain.PermCategoryManage,
					domain.PermItemCreate,
					domain.PermItemEdit,
				))
				g.Post("/categories", cats.create)
			})
			secure.Group(func(g chi.Router) {
				g.Use(requireGlobal(domain.PermCategoryManage))
				g.Patch("/categories/{id}", cats.update)
				g.Delete("/categories/{id}", cats.delete)
			})
			secure.Group(func(g chi.Router) {
				g.Use(requireAnyGlobal(
					domain.PermLocationManage,
					domain.PermItemCreate,
					domain.PermItemEdit,
				))
				g.Post("/locations", locs.create)
			})
			secure.Group(func(g chi.Router) {
				g.Use(requireGlobal(domain.PermLocationManage))
				g.Patch("/locations/{id}", locs.update)
				g.Delete("/locations/{id}", locs.delete)
			})

			secure.Route("/items", items.routes)
			secure.Route("/signouts", signouts.routes)
			secure.Route("/lostfound", lostfound.routes)
			secure.Get("/dashboard", dashboardHandler(deps.Pool))
			// /settings: anyone authenticated can read; only admins can patch.
			// Split here so read and patch can use different middleware (chi Route
			// would apply one middleware block to all verbs on that sub-router).
			secure.Get("/settings", settings.get)
			secure.Group(func(g chi.Router) {
				g.Use(requireGlobal(domain.PermAccessManage))
				g.Patch("/settings", settings.patch)
			})
			secure.Route("/audit-log", auditLogHandler(deps.Pool))
			secure.Get("/export/items.csv", exportItemsCSV(deps.Pool))
			secure.Get("/export/signouts.csv", exportSignoutsCSV(deps.Pool))
			secure.Route("/files", fileH.routes)
		})
	})

	return r
}

func meHandler(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.FromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	memberships := make([]map[string]any, 0, len(u.Memberships))
	for _, m := range u.Memberships {
		var dept *string
		if m.DepartmentID != nil {
			s := m.DepartmentID.String()
			dept = &s
		}
		memberships = append(memberships, map[string]any{
			"department_id": dept,
			"role":          m.Role,
		})
	}
	// Effective permissions (global + dept-scoped union) so the SPA can
	// gate navigation; precise dept-aware enforcement stays server-side.
	perms := u.EffectivePermissions()
	if perms == nil {
		perms = []string{}
	}
	primary := ""
	if pd := u.PrimaryDepartment(); pd != nil {
		primary = pd.String()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":                    u.ID.String(),
		"display_name":          u.DisplayName,
		"email":                 u.Email,
		"oidc_subject":          u.OIDCSubject,
		"memberships":           memberships,
		"permissions":           perms,
		"primary_department_id": primary,
	})
}
