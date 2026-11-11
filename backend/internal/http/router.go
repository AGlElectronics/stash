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
	"github.com/AGlElectronics/stash/backend/internal/clientip"
	"github.com/AGlElectronics/stash/backend/internal/config"
	"github.com/AGlElectronics/stash/backend/internal/denlink"
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
	// Den is the pairing with Den; nil means a fresh standalone client.
	Den *denlink.Client
	// Telegram is the Lost & Found bot; nil means no bot.
	Telegram *service.LostFoundTelegram
	// Overdue lists overdue sign-outs; nil builds one that sends no email.
	Overdue *service.OverdueService
}

// NewRouter wires the chi router with middleware, CORS and routes.
// Version is exposed via /api/health.
func NewRouter(deps Deps, version string) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(clientip.Middleware(deps.Config.TrustedProxyCIDRs))
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
	fileH := &fileHandler{svc: deps.Files}
	den := deps.Den
	if den == nil {
		den = denlink.New(deps.Pool, denlink.Config{Slug: "stash", Version: version, Mode: deps.Config.DenMode}, deps.Logger)
	}
	settingsSvc := service.NewSettingsService(deps.Pool).WithDen(service.DenSource(den))
	settings := &settingsHandler{svc: settingsSvc}
	bot := deps.Telegram
	if bot == nil {
		bot = service.NewLostFoundTelegram(deps.Pool, settingsSvc, nil, deps.Logger)
	}
	overdue := deps.Overdue
	if overdue == nil {
		overdue = service.NewOverdueService(deps.Pool, settingsSvc, nil, "", "", deps.Logger)
	}
	lostfound := &lostFoundHandler{svc: service.NewLostFoundService(deps.Pool), telegram: &telegramHandler{bot: bot}}
	users := &usersHandler{svc: service.NewUserService(deps.Pool)}

	publicLF := &publicLostFoundHandler{pool: deps.Pool}

	r.Route("/api", func(api chi.Router) {
		api.Use(den.RequirePaired)
		api.Get("/health", health.get)
		api.Get("/role-catalog", roleCatalogHandler(deps.Access))
		api.Get("/brand", den.BrandHandler("/api/brand"))
		api.Get("/brand/{role}", den.AssetHandler)
		api.Get("/den/state", den.StateHandler)

		if deps.Config.PublicLostFoundClaimEnabled {
			api.Route("/public/lostfound", func(pub chi.Router) {
				pub.Get("/", publicLF.search)
				pub.Get("/{reportNumber}", publicLF.getByNumber)
			})
		}

		api.Group(func(secure chi.Router) {
			secure.Use(authMW)
			secure.Get("/me", meHandler)
			prefs := den.PreferencesHandler(func(r *http.Request) (string, string, bool) {
				u, ok := auth.FromContext(r.Context())
				if !ok || u.OIDCSubject == "" || deps.Config.OIDCIssuerURL == "" {
					return "", "", false
				}
				return deps.Config.OIDCIssuerURL, u.OIDCSubject, true
			})
			secure.Get("/me/preferences", prefs)
			secure.Put("/me/preferences", prefs)

			// Reference data: anyone who can see items can read it; only the
			// matching manage permission may create, rename or delete. Item
			// roles pick from existing entries and never create them.
			secure.Group(func(g chi.Router) {
				g.Use(requireAnyOf(domain.PermItemView, domain.PermSignoutView))
				g.Get("/departments", depts.list)
				g.Get("/departments/{id}", depts.get)
				g.Get("/categories", cats.list)
				g.Get("/categories/{id}", cats.get)
				g.Get("/locations", locs.list)
				g.Get("/locations/{id}", locs.get)
			})
			secure.Group(func(g chi.Router) {
				g.Use(require(domain.PermDepartmentManage))
				g.Post("/departments", depts.create)
				g.Patch("/departments/{id}", depts.update)
				g.Delete("/departments/{id}", depts.delete)
			})
			secure.Group(func(g chi.Router) {
				g.Use(require(domain.PermCategoryManage))
				g.Post("/categories", cats.create)
				g.Patch("/categories/{id}", cats.update)
				g.Delete("/categories/{id}", cats.delete)
			})
			secure.Group(func(g chi.Router) {
				g.Use(require(domain.PermLocationManage))
				g.Post("/locations", locs.create)
				g.Patch("/locations/{id}", locs.update)
				g.Delete("/locations/{id}", locs.delete)
			})

			secure.Route("/items", items.routes)
			secure.Route("/signouts", signouts.routes)
			secure.Route("/lostfound", lostfound.routes)
			secure.Get("/dashboard", dashboardHandler(deps.Pool))
			secure.Get("/notifications", notificationsHandler(overdue))
			secure.Route("/users", users.routes)
			// /settings: anyone authenticated can read; only admins can patch.
			// Split here so read and patch can use different middleware (chi Route
			// would apply one middleware block to all verbs on that sub-router).
			secure.Get("/settings", settings.get)
			secure.Group(func(g chi.Router) {
				g.Use(require(domain.PermAccessManage))
				g.Patch("/settings", settings.patch)
				g.Route("/den", func(dr chi.Router) {
					dr.Get("/", den.StatusHandler)
					dr.Post("/pair", auditDen(deps.Pool, den, "den.pair", den.PairHandler))
					dr.Post("/sync", auditDen(deps.Pool, den, "den.sync", den.SyncHandler))
					dr.Delete("/pair", auditDen(deps.Pool, den, "den.unpair", den.UnpairHandler))
				})
			})
			secure.Route("/audit-log", auditLogHandler(deps.Pool))
			secure.Get("/export/items.csv", exportItemsCSV(deps.Pool))
			secure.Get("/export/signouts.csv", exportSignoutsCSV(deps.Pool))
			secure.Get("/export/lostfound.csv", exportLostFoundCSV(deps.Pool))
			secure.With(require(domain.PermExportCreate)).Get("/export/reports", listPlannedReports)
			secure.Get("/export/reports/{report}.csv", exportPlannedReport(deps.Pool))
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
	roles := make([]string, 0, len(u.Roles))
	for _, role := range u.Roles {
		roles = append(roles, auth.RoleKey(role))
	}
	perms := u.EffectivePermissions()
	if perms == nil {
		perms = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":           u.ID.String(),
		"display_name": u.DisplayName,
		"email":        u.Email,
		"oidc_subject": u.OIDCSubject,
		"roles":        roles,
		"permissions":  perms,
	})
}
