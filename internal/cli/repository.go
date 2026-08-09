package cli

import (
	"fmt"

	"github.com/lean-tech/git-rodolfo/internal/app"
)

// bindRepositoryIdentity resolves the current working directory, checks
// it's actually a Git repository (§21's "the directory is not a
// repository" case, shared by "use" and "current"), and builds a
// RepositoryIdentityService bound to it.
func bindRepositoryIdentity(deps Deps) (*app.RepositoryIdentityService, string, int) {
	dir, err := deps.Getwd()
	if err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return nil, "", 1
	}

	git := deps.NewGitClient(dir)
	if !git.IsRepository(dir) {
		fmt.Fprintln(deps.Stderr, "The current directory is not a Git repository.")
		fmt.Fprintln(deps.Stderr)
		fmt.Fprintln(deps.Stderr, "Run this command inside a repository or use:")
		fmt.Fprintln(deps.Stderr, "git rodolfo clone <repository-url>")
		return nil, "", 1
	}

	svc := app.NewRepositoryIdentityService(git, deps.Mechanism, deps.AccountRepo)
	return svc, dir, 0
}
