package service

import "github.com/AGlElectronics/stash/backend/internal/denlink"

type denSource struct{ c *denlink.Client }

// DenSource reads Stash's Den-managed settings (the organisation name and
// Den's module settings) from the bundle the Den link keeps.
func DenSource(c *denlink.Client) SettingsSource {
	if c == nil {
		return nil
	}
	return denSource{c}
}

func (d denSource) Paired() bool             { return d.c.Paired() }
func (d denSource) OrganizationName() string { return d.c.OrganizationName() }

func (d denSource) ModuleSettings() *denlink.ModuleSettings { return d.c.ModuleSettings() }
