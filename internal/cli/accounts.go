package cli

import (
	"fmt"
	"text/tabwriter"
)

// runAccounts implements "git rodolfo accounts" (RF-02, PRD §13.1): a
// read-only list, never a menu.
func runAccounts(deps Deps, args []string) int {
	if len(args) > 0 {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: \"accounts\" takes no arguments (got %q)\n", args[0])
		return 1
	}

	accounts, err := deps.Accounts.List()
	if err != nil {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: %v\n", err)
		return 1
	}

	if len(accounts) == 0 {
		fmt.Fprintln(deps.Stdout, "No accounts registered.")
		return 0
	}

	fmt.Fprintln(deps.Stdout, "Git accounts:")
	fmt.Fprintln(deps.Stdout)

	w := tabwriter.NewWriter(deps.Stdout, 0, 0, 3, ' ', 0)
	for _, a := range accounts {
		fmt.Fprintf(w, "  %s\t%s\t%s\n", a.DisplayName, a.GitEmail, listStatusText(a))
	}
	w.Flush()

	fmt.Fprintln(deps.Stdout)
	fmt.Fprintf(deps.Stdout, "%s. Run \"git rodolfo account show <account>\" for details.\n",
		pluralize(len(accounts), "account"))
	return 0
}
