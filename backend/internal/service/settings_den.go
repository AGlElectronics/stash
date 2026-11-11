package service

import "github.com/AGlElectronics/stash/backend/internal/denlink"

type denSource struct{ c *denlink.Client }

// DenSource reads Stash's Den-managed settings from the integration bundle
// the Den link keeps in memory.
func DenSource(c *denlink.Client) SettingsSource {
	if c == nil {
		return nil
	}
	return denSource{c}
}

func (d denSource) Paired() bool             { return d.c.Paired() }
func (d denSource) OrganizationName() string { return d.c.OrganizationName() }

func (d denSource) DenSettings() map[string]any {
	if b := d.c.Bundle(); b != nil {
		return b.Settings.Data
	}
	return nil
}
