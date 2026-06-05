package clanker

import "github.com/nicomaioli-skinandme/dotfiles/sam/internal/session"

// Controller exposes the clanker actions. Its only cross-entity edge is to
// the session entity, used to annotate whether each clanker's tmux session
// is live. Attaching to a clanker's session is the session entity's job
// (AttachExisting), so the Controller surfaces only listing.
type Controller struct {
	clankers Service
	sessions session.Service
}

// NewController returns a clanker Controller backed by the given services.
func NewController(clankers Service, sessions session.Service) Controller {
	return Controller{clankers: clankers, sessions: sessions}
}

// List returns the running clankers with Active annotated from the session
// entity.
func (c Controller) List() ([]Clanker, error) {
	ks, err := c.clankers.List()
	if err != nil {
		return nil, err
	}
	for i := range ks {
		if ks[i].Session != "" {
			ks[i].Active = c.sessions.Has(ks[i].Session)
		}
	}
	return ks, nil
}
