package auth

import (
	"fmt"
	"sort"
	"strings"
)

// GroupBinding maps an OIDC group claim to a Stash role. Bindings live in
// the access config file mounted into the backend pod (see accessconfig.go)
// and are always global.
type GroupBinding struct {
	OIDCGroup string
	RoleName  string
}

// GroupBindings is an immutable snapshot of the OIDC group → role mapping
// loaded once at startup from the access config file. It is safe to read
// concurrently: the maps are never mutated after construction.
type GroupBindings struct {
	byGroup map[string][]GroupBinding
	all     []GroupBinding
}

// Empty reports whether the snapshot contains no bindings.
func (gb *GroupBindings) Empty() bool {
	return gb == nil || len(gb.all) == 0
}

// All returns every binding sorted by (group, role).
func (gb *GroupBindings) All() []GroupBinding {
	if gb == nil {
		return nil
	}
	out := make([]GroupBinding, len(gb.all))
	copy(out, gb.all)
	return out
}

// Lookup returns every binding that matches any of the supplied groups,
// deduplicated by (group, role). Groups are compared exactly.
func (gb *GroupBindings) Lookup(groups []string) []GroupBinding {
	if gb == nil || len(groups) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := []GroupBinding{}
	for _, g := range groups {
		for _, b := range gb.byGroup[g] {
			key := b.OIDCGroup + "\x00" + b.RoleName
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, b)
		}
	}
	return out
}

type bindingFileEntry struct {
	// Group is the OIDC group name (e.g. "stash-admin"); it must match the
	// claim coming from the IdP exactly.
	Group string `toml:"group"`
	// Role is the Stash role name (e.g. "admin", "frontdesk").
	Role string `toml:"role"`
	// DepartmentID is no longer supported. It is still parsed so a file
	// from an earlier version fails with a clear message.
	DepartmentID string `toml:"department_id,omitempty"`
}

// parseGroupBindings parses a TOML document containing only [[bindings]].
// Kept for tests; production loading goes through LoadAccessConfig.
func parseGroupBindings(data []byte) (*GroupBindings, error) {
	cfg, err := parseAccessConfig(data)
	if err != nil {
		return nil, err
	}
	return cfg.Bindings, nil
}

// buildGroupBindings validates and indexes the binding entries of the access
// config file.
func buildGroupBindings(entries []bindingFileEntry) (*GroupBindings, error) {
	out := &GroupBindings{byGroup: map[string][]GroupBinding{}}
	seen := map[string]struct{}{}
	for i, e := range entries {
		group := strings.TrimSpace(e.Group)
		role := strings.TrimSpace(e.Role)
		if group == "" {
			return nil, fmt.Errorf("binding[%d]: group is required", i)
		}
		if role == "" {
			return nil, fmt.Errorf("binding[%d] (group=%q): role is required", i, group)
		}
		if strings.TrimSpace(e.DepartmentID) != "" {
			return nil, fmt.Errorf("binding[%d] (group=%q): department_id is not supported; roles are global, remove the line", i, group)
		}
		b := GroupBinding{OIDCGroup: group, RoleName: role}
		key := b.OIDCGroup + "\x00" + b.RoleName
		if _, dup := seen[key]; dup {
			return nil, fmt.Errorf("binding[%d]: duplicate (group=%q, role=%q)", i, group, role)
		}
		seen[key] = struct{}{}

		out.byGroup[group] = append(out.byGroup[group], b)
		out.all = append(out.all, b)
	}

	sort.Slice(out.all, func(i, j int) bool {
		if out.all[i].OIDCGroup != out.all[j].OIDCGroup {
			return out.all[i].OIDCGroup < out.all[j].OIDCGroup
		}
		return out.all[i].RoleName < out.all[j].RoleName
	})
	return out, nil
}
