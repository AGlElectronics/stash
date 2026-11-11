package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/AGlElectronics/stash/backend/internal/config"
	"github.com/AGlElectronics/stash/backend/internal/groupmatch"
)

// OIDCVerifier checks the access tokens Stash's web app sends (token
// contract, docs/concepts/authentication.md): signature, issuer, expiry and
// an audience naming Stash's API (OIDC_AUDIENCE, default stash-api). ID
// tokens are refused.
type OIDCVerifier struct {
	verifier *oidc.IDTokenVerifier
	groups   groupmatch.Config
}

// OIDCClaims is the parsed claim subset Stash uses.
type OIDCClaims struct {
	Subject           string `json:"sub"`
	Email             string `json:"email"`
	PreferredUsername string `json:"preferred_username"`
	Name              string `json:"name"`
	Nickname          string `json:"nickname"`
	// Type is Keycloak's "typ" claim: "Bearer" for access tokens, "ID" for
	// ID tokens.
	Type string `json:"typ"`
	// Groups come only from OIDC_GROUPS_CLAIM (groupmatch); "roles" is
	// never merged in.
	Groups []string `json:"-"`
}

// NewOIDCVerifier constructs the verifier from config. It performs OIDC
// discovery against the issuer URL and uses the result for JWKS rotation.
func NewOIDCVerifier(ctx context.Context, cfg *config.Config) (*OIDCVerifier, error) {
	if cfg.OIDCIssuerURL == "" || cfg.OIDCClientID == "" {
		return nil, errors.New("OIDC verifier requires OIDC_ISSUER_URL and OIDC_CLIENT_ID")
	}
	provider, err := oidc.NewProvider(ctx, cfg.OIDCIssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	return &OIDCVerifier{
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.Audience()}),
		groups:   cfg.GroupMatch,
	}, nil
}

// Verify parses & validates a bearer token, returning the claims subset we
// care about. The caller is responsible for then calling Resolver.
func (v *OIDCVerifier) Verify(ctx context.Context, bearer string) (*OIDCClaims, error) {
	bearer = strings.TrimSpace(bearer)
	if bearer == "" {
		return nil, errors.New("missing bearer token")
	}
	token, err := v.verifier.Verify(ctx, bearer)
	if err != nil {
		return nil, fmt.Errorf("verify token: %w", err)
	}
	var c OIDCClaims
	if err := token.Claims(&c); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}
	if !strings.EqualFold(c.Type, "Bearer") {
		return nil, fmt.Errorf("token type %q: the API needs an access token", c.Type)
	}
	if c.Subject == "" {
		return nil, errors.New("token missing sub claim")
	}
	var all map[string]any
	if err := token.Claims(&all); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}
	c.Groups = v.groups.Groups(all)
	return &c, nil
}
