package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/AGlElectronics/stash/backend/internal/config"
)

// OIDCVerifier wraps the go-oidc verifier and exposes the subset of token
// claims we use for authentication and group→role mapping.
type OIDCVerifier struct {
	verifier *oidc.IDTokenVerifier
	cfg      *config.Config
}

// OIDCClaims is the parsed shape we accept from the IdP. We try the standard
// claim names first and fall back to the Keycloak-specific shapes used by
// older realms.
type OIDCClaims struct {
	Subject           string   `json:"sub"`
	Email             string   `json:"email"`
	PreferredUsername string   `json:"preferred_username"`
	Name              string   `json:"name"`
	Nickname          string   `json:"nickname"`
	Groups            []string `json:"groups"`
	Roles             []string `json:"roles"`
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
	audience := cfg.OIDCAudience
	if audience == "" {
		audience = cfg.OIDCClientID
	}
	v := provider.Verifier(&oidc.Config{ClientID: audience})
	return &OIDCVerifier{verifier: v, cfg: cfg}, nil
}

// Verify parses & validates a bearer token, returning the claims subset we
// care about. The caller is responsible for then calling Resolver.
func (v *OIDCVerifier) Verify(ctx context.Context, bearer string) (*OIDCClaims, error) {
	bearer = strings.TrimSpace(bearer)
	if bearer == "" {
		return nil, errors.New("missing bearer token")
	}
	idToken, err := v.verifier.Verify(ctx, bearer)
	if err != nil {
		return nil, fmt.Errorf("verify token: %w", err)
	}
	var c OIDCClaims
	if err := idToken.Claims(&c); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}
	if c.Subject == "" {
		return nil, errors.New("token missing sub claim")
	}
	// Many IdPs do not expose plain `groups`; allow either claim. We treat
	// roles and groups as equivalent inputs for the binding mapper - admins
	// can choose which to use.
	c.Groups = mergeUnique(c.Groups, c.Roles)
	return &c, nil
}

func mergeUnique(a, b []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(a)+len(b))
	for _, v := range a {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	for _, v := range b {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
