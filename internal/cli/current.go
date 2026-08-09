package cli

import (
	"fmt"
	"path/filepath"
)

// runCurrent implements "git rodolfo current" (§13.8): distinguishes a
// repository not managed by Git Rodolfo from one managed but drifted.
func runCurrent(deps Deps, args []string) int {
	if len(args) > 0 {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: \"current\" takes no arguments (got %q)\n", args[0])
		return 1
	}

	svc, dir, code := bindRepositoryIdentity(deps)
	if code != 0 {
		return code
	}
	repoName := filepath.Base(dir)

	status, err := svc.Current()
	if err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}

	if !status.Managed {
		fmt.Fprintf(deps.Stdout, "Repository: %s\n\n", repoName)
		fmt.Fprintln(deps.Stdout, "This repository is not managed by Git Rodolfo.")
		if status.EffectiveGitEmail != "" {
			fmt.Fprintf(deps.Stdout, "It will use your global Git identity: %s\n", status.EffectiveGitEmail)
		}
		fmt.Fprintln(deps.Stdout)
		fmt.Fprintln(deps.Stdout, "Run:")
		fmt.Fprintln(deps.Stdout, "git rodolfo use")
		return 0
	}

	if status.AccountDeleted {
		fmt.Fprintf(deps.Stdout, "Repository: %s\n\n", repoName)
		fmt.Fprintf(deps.Stdout, "This repository is assigned to account %q, which no longer exists.\n", status.AssignedID)
		fmt.Fprintln(deps.Stdout)
		fmt.Fprintln(deps.Stdout, "Run:")
		fmt.Fprintln(deps.Stdout, "git rodolfo use")
		return 0
	}

	fmt.Fprintf(deps.Stdout, "Repository: %s\n", repoName)
	fmt.Fprintf(deps.Stdout, "Account: %s\n", status.Account.DisplayName)
	fmt.Fprintf(deps.Stdout, "Commit name: %s\n", status.Account.GitName)
	fmt.Fprintf(deps.Stdout, "Commit email: %s\n", status.Account.GitEmail)
	if status.Account.PrivateKeyPath != "" {
		fmt.Fprintf(deps.Stdout, "SSH key: %s\n", status.Account.PrivateKeyPath)
	}
	if status.HasRemote {
		fmt.Fprintf(deps.Stdout, "Remote: %s\n", status.RemoteURL)
	} else {
		fmt.Fprintln(deps.Stdout, "Remote: none")
	}
	fmt.Fprintln(deps.Stdout)

	if status.Validation.OK {
		fmt.Fprintln(deps.Stdout, "Status: correctly configured")
		return 0
	}

	fmt.Fprintln(deps.Stdout, "Warning:")
	for _, issue := range status.Validation.Issues {
		fmt.Fprintf(deps.Stdout, "  - %s\n", issue)
	}
	fmt.Fprintln(deps.Stdout)
	fmt.Fprintln(deps.Stdout, "Run:")
	fmt.Fprintf(deps.Stdout, "git rodolfo use %s\n", status.Account.ID)
	return 0
}
