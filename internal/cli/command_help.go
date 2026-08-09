package cli

// commandHelpText holds the "<command> --help" / "<command> -h" text for
// each command and, for "account", each of its subcommands. Keyed by the
// command words joined with a space (e.g. "account show"), matching what
// commandHelpKey derives from the parsed arguments.
var commandHelpText = map[string]string{
	"accounts": `Usage: git-rodolfo accounts

List every registered account (read-only — this never opens a menu or
changes state; use the subcommands below for that).

Example:
  git-rodolfo accounts
`,

	"account": `Usage: git-rodolfo account <show|add|edit|remove> ...

Manage registered accounts. Run "git-rodolfo account <subcommand> --help"
for details on a specific one.

Subcommands:
  show <account>      Show details for one account
  add                 Register a new account
  edit [account]      Edit a registered account
  remove [account]    Remove a registered account
`,

	"account show": `Usage: git-rodolfo account show <account>

Show an account's details: provider, username, commit name/email, SSH key
path, and authentication status (never the private key's contents).

Example:
  git-rodolfo account show lean-tech
`,

	"account add": `Usage: git-rodolfo account add [flags]

Register a new account. With no flags, runs an interactive wizard. Pass
--non-interactive with the flags below to run it unattended (e.g. from a
script); exactly one of --key, --generate-key or --configure-later is
required in that mode.

Flags (require --non-interactive):
  --name <name>              Account display name
  --username <username>      Provider (GitHub) username
  --commit-name <name>       Git commit name
  --commit-email <email>     Git commit email
  --key <path>                Associate an existing private key
  --generate-key               Generate a new Ed25519 key
  --key-path <path>            Target path for --generate-key
                                (default: ~/.ssh/id_ed25519_<account-id>)
  --overwrite                  Allow --generate-key to overwrite an
                                existing file at --key-path
  --configure-later            Register without an SSH key yet
  --register-key-with-gh       Register the public key on GitHub via gh
                                (requires --key or --generate-key)

Examples:
  git-rodolfo account add
  git-rodolfo account add --non-interactive \
    --name "Lean Tech" --username cristhiandelgado-work \
    --commit-name "Cristhian Delgado" --commit-email cristhian@leantech.com \
    --generate-key
`,

	"account edit": `Usage: git-rodolfo account edit [account] [flags]

Edit a registered account. With no account id, shows a selector (unless
--non-interactive, which requires the id). Only the fields you pass
change; the rest stay as they are.

Flags (require --non-interactive):
  --display-name <name>      New display name
  --commit-name <name>       New commit name
  --commit-email <email>     New commit email
  --username <username>      New provider username
  --key <path>                Associate a different existing private key
  --delete-old-key             Delete the previous key's files once --key
                                replaces it

Examples:
  git-rodolfo account edit lean-tech
  git-rodolfo account edit lean-tech --non-interactive --commit-email new@leantech.com
`,

	"account remove": `Usage: git-rodolfo account remove [account] [--delete-key]

Remove a registered account's local profile. Removing the profile and
deleting its SSH key files are two independent confirmations — the key is
never deleted by default, and never by --yes alone.

Flags:
  --delete-key    Also delete the account's SSH key files (requires --yes
                   in --non-interactive mode; not offered if the key is
                   still used by another registered account)

Examples:
  git-rodolfo account remove lean-tech
  git-rodolfo account remove lean-tech --non-interactive --yes --delete-key
`,

	"clone": `Usage: git-rodolfo clone <repository-url> [--account <account>] [--keep-https]

Clone a GitHub repository, forcing the chosen account's SSH key from the
first connection — this is what keeps another loaded key in ssh-agent from
being offered instead.

Flags:
  --account <account>    Account to clone with (shows a selector if omitted,
                          required with --non-interactive)
  --keep-https             For an HTTPS URL, keep it as-is instead of
                            switching to SSH (asked interactively otherwise)

Examples:
  git-rodolfo clone git@github.com:lean-tech/project.git --account lean-tech
  git-rodolfo clone https://github.com/lean-tech/project.git --keep-https --account lean-tech --non-interactive
`,

	"use": `Usage: git-rodolfo use [account] [--clear]

Assign an account's identity to the current repository (works with or
without a remote), or remove every value Git Rodolfo set with --clear.

Flags:
  --clear    Remove user.name/user.email/rodolfo.account/core.sshCommand
              set by Git Rodolfo, reverting to Git's default behavior

Examples:
  git-rodolfo use lean-tech
  git-rodolfo use --clear
`,

	"current": `Usage: git-rodolfo current

Show which account (if any) the current repository is using, and whether
its local Git configuration actually matches that account.

Example:
  git-rodolfo current
`,

	"doctor": `Usage: git-rodolfo doctor

Diagnose Git/SSH/GitHub configuration issues across every registered
account (and, if run inside one, the current repository). Reports and
suggests a fix for each issue found; never changes anything itself.

Example:
  git-rodolfo doctor
`,

	"completion": `Usage: git-rodolfo completion <bash|zsh|fish>

Print a shell completion script to stdout for the given shell.

Examples:
  # bash (add to ~/.bashrc, or a file sourced from it):
  git-rodolfo completion bash > ~/.local/share/bash-completion/completions/git-rodolfo

  # zsh (add to a directory on your $fpath, then restart your shell):
  git-rodolfo completion zsh > "${fpath[1]}/_git-rodolfo"

  # fish:
  git-rodolfo completion fish > ~/.config/fish/completions/git-rodolfo.fish
`,

	"update": `Usage: git-rodolfo update [--version <tag>]

Download the latest git-rodolfo release (or a specific one with
--version) and replace the currently running binary. Requires
confirmation unless --yes is given.

Flags:
  --version <tag>    Update to this exact release tag instead of the
                       latest (e.g. v0.2.0)

Examples:
  git-rodolfo update
  git-rodolfo update --version v0.2.0
  git-rodolfo update --non-interactive --yes
`,
}

// hasHelpFlag reports whether --help or -h appears anywhere in args.
func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}

// commandHelpKey derives the commandHelpText key for the parsed top-level
// arguments — "account show" for ["account", "show", ...], "clone" for
// ["clone", ...], and so on.
func commandHelpKey(rest []string) string {
	if len(rest) == 0 {
		return ""
	}
	if rest[0] == "account" && len(rest) > 1 {
		switch rest[1] {
		case "show", "add", "edit", "remove":
			return "account " + rest[1]
		}
	}
	return rest[0]
}
