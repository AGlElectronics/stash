package auth

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/AGlElectronics/stash/backend/internal/config"
	"github.com/AGlElectronics/stash/backend/internal/httperror"
)

// Middleware authenticates the request based on cfg.AuthMode. In dev mode it
// reads the X-Dev-User header (or DEV_DEFAULT_USER), "name" for an admin or
// "name:role1,role2" for other roles, and resolves a User. In
// OIDC mode it verifies the bearer JWT, then provisions/refreshes the user's
// memberships based on the IdP-supplied groups/roles claim.
func Middleware(cfg *config.Config, resolver *Resolver, oidcVerifier *OIDCVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			switch cfg.AuthMode {
			case config.AuthModeDev:
				header := r.Header.Get("X-Dev-User")
				if header == "" {
					header = cfg.DevDefaultUser
				}
				user, err := resolver.LoadOrProvisionDevUser(ctx, header)
				if err != nil {
					log.Ctx(ctx).Warn().Err(err).Msg("dev auth failed")
					httperror.Render(w, r, err)
					return
				}
				ctx = WithUser(ctx, user)

			case config.AuthModeOIDC:
				if oidcVerifier == nil {
					httperror.Render(w, r, httperror.Internal(nil))
					return
				}
				bearer := bearerFromHeader(r.Header.Get("Authorization"))
				if bearer == "" {
					httperror.Render(w, r, httperror.New(httperror.CodeUnauthorized, "missing bearer token"))
					return
				}
				claims, err := oidcVerifier.Verify(ctx, bearer)
				if err != nil {
					log.Ctx(ctx).Debug().Err(err).Msg("oidc verify failed")
					httperror.Render(w, r, httperror.New(httperror.CodeUnauthorized, "invalid token"))
					return
				}
				user, err := resolver.LoadOrProvisionOIDCUser(ctx, claims)
				if err != nil {
					log.Ctx(ctx).Error().Err(err).Msg("oidc resolver failed")
					httperror.Render(w, r, httperror.Internal(err))
					return
				}
				ctx = WithUser(ctx, user)

			default:
				httperror.Render(w, r, httperror.Internal(nil))
				return
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerFromHeader(h string) string {
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
