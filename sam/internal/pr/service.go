package pr

import (
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/config"
	"github.com/nicomaioli-skinandme/dotfiles/sam/internal/ghx"
)

// Service holds the PR primitives. Infra-only (ghx); the zero value is
// ready to use.
type Service struct{}

// List returns the open PRs in the workspace's branch repo that request the
// current gh user as a reviewer.
func (Service) List(ws *config.Workspace) ([]PR, error) {
	prs, err := ghx.PRsForReview(ws.BranchRepo)
	if err != nil {
		return nil, err
	}
	out := make([]PR, len(prs))
	for i, p := range prs {
		out[i] = fromGh(p)
	}
	return out, nil
}

// ByFlag resolves a specific PR for the non-interactive CLI path.
func (Service) ByFlag(repo string, num int) (PR, error) {
	p, err := ghx.PRView(repo, num)
	if err != nil {
		return PR{}, err
	}
	return fromGh(p), nil
}
