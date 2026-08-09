package cli

import "fmt"

// runCompletion implements "git-rodolfo completion <bash|zsh|fish>":
// prints a static completion script to stdout for the given shell,
// covering top-level commands, "account"'s subcommands, and
// "completion"'s own shell names. It doesn't complete account IDs
// dynamically (that would mean shelling back out to git-rodolfo from
// inside the completion function) — a reasonable follow-up if it turns
// out to matter in practice, but not required for tab-completing the
// command structure itself.
func runCompletion(deps Deps, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(deps.Stderr, "git-rodolfo: \"completion\" requires a shell: bash, zsh or fish")
		return 1
	}
	if len(args) > 1 {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: \"completion\" takes a single shell name (got %d)\n", len(args))
		return 1
	}

	script, ok := completionScripts[args[0]]
	if !ok {
		fmt.Fprintf(deps.Stderr, "git-rodolfo: unsupported shell %q (expected bash, zsh or fish)\n", args[0])
		return 1
	}
	fmt.Fprint(deps.Stdout, script)
	return 0
}

var completionScripts = map[string]string{
	"bash": bashCompletionScript,
	"zsh":  zshCompletionScript,
	"fish": fishCompletionScript,
}

const (
	bashCompletionScript = `# git-rodolfo bash completion
# Install: git-rodolfo completion bash > ~/.local/share/bash-completion/completions/git-rodolfo
_git_rodolfo_completions() {
  local cur prev
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"

  local commands="accounts account clone use current doctor completion update help"
  local account_subcommands="show add edit remove"
  local global_flags="--non-interactive --yes --verbose --help -h --version -v"

  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=($(compgen -W "${commands}" -- "${cur}"))
    return 0
  fi

  if [[ "${COMP_WORDS[1]}" == "account" && ${COMP_CWORD} -eq 2 ]]; then
    COMPREPLY=($(compgen -W "${account_subcommands}" -- "${cur}"))
    return 0
  fi

  if [[ "${COMP_WORDS[1]}" == "completion" && ${COMP_CWORD} -eq 2 ]]; then
    COMPREPLY=($(compgen -W "bash zsh fish" -- "${cur}"))
    return 0
  fi

  COMPREPLY=($(compgen -W "${global_flags}" -- "${cur}"))
}
complete -F _git_rodolfo_completions git-rodolfo
`

	zshCompletionScript = `#compdef git-rodolfo
# git-rodolfo zsh completion
# Install: git-rodolfo completion zsh > "${fpath[1]}/_git-rodolfo" (then restart your shell)
_git_rodolfo() {
  local -a commands
  commands=(
    'accounts:List registered accounts'
    'account:Manage accounts (show, add, edit, remove)'
    'clone:Clone a repository with a chosen account'
    'use:Assign or clear an account for this repository'
    'current:Show which account this repository is using'
    'doctor:Diagnose Git/SSH/GitHub configuration issues'
    'completion:Generate a shell completion script'
    'update:Update git-rodolfo itself'
    'help:Show help'
  )

  if (( CURRENT == 2 )); then
    _describe 'command' commands
    return
  fi

  case ${words[2]} in
    account)
      local -a subcommands
      subcommands=(
        'show:Show details for one account'
        'add:Register a new account'
        'edit:Edit a registered account'
        'remove:Remove a registered account'
      )
      _describe 'account subcommand' subcommands
      ;;
    completion)
      local -a shells
      shells=(bash zsh fish)
      _describe 'shell' shells
      ;;
  esac
}
_git_rodolfo "$@"
`

	fishCompletionScript = `# git-rodolfo fish completion
# Install: git-rodolfo completion fish > ~/.config/fish/completions/git-rodolfo.fish
complete -c git-rodolfo -f -n '__fish_use_subcommand' -a accounts -d 'List registered accounts'
complete -c git-rodolfo -f -n '__fish_use_subcommand' -a account -d 'Manage accounts (show, add, edit, remove)'
complete -c git-rodolfo -f -n '__fish_use_subcommand' -a clone -d 'Clone a repository with a chosen account'
complete -c git-rodolfo -f -n '__fish_use_subcommand' -a use -d 'Assign or clear an account for this repository'
complete -c git-rodolfo -f -n '__fish_use_subcommand' -a current -d 'Show which account this repository is using'
complete -c git-rodolfo -f -n '__fish_use_subcommand' -a doctor -d 'Diagnose Git/SSH/GitHub configuration issues'
complete -c git-rodolfo -f -n '__fish_use_subcommand' -a completion -d 'Generate a shell completion script'
complete -c git-rodolfo -f -n '__fish_use_subcommand' -a update -d 'Update git-rodolfo itself'
complete -c git-rodolfo -f -n '__fish_use_subcommand' -a help -d 'Show help'

complete -c git-rodolfo -f -n '__fish_seen_subcommand_from account' -a 'show add edit remove'
complete -c git-rodolfo -f -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'
`
)
