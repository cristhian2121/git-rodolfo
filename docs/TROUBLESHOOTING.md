# Troubleshooting

Start with `git rodolfo doctor` — it catches most of what's below on its
own and tells you the exact command to run. This guide is for the cases it
can't fix itself, plus some background on *why*.

## "It authenticated as the wrong account"

This is exactly the failure mode Git Rodolfo exists to prevent (see the
PRD §4.2/§12.2), so if you hit it on a repository Git Rodolfo actually
manages, something's overriding its configuration:

1. `env | grep GIT_SSH_COMMAND` — if set, it silently overrides
   `core.sshCommand` for every repository, with no warning from Git
   itself. `doctor` catches this (validation #8); the fix is
   `unset GIT_SSH_COMMAND`.
2. `git config --local --list` inside the repo — confirm `core.sshcommand`
   actually points at the key you expect. If it's missing entirely, the
   repo was never configured with `git rodolfo use` or `clone` — run one
   of them.
3. Confirm you're actually inside the repo you think you are (`current`).

## "`doctor` says a key is missing" but the file is right there

`doctor` checks the exact path stored in `config.json`
(`~/.config/git-rodolfo/config.json`, or `$XDG_CONFIG_HOME/git-rodolfo/config.json`
if that's set) — usually an absolute path with `~` already expanded. A
key moved, renamed, or restored from a backup to a different path won't
match. Fix with `git rodolfo account edit <account> --key <new-path>`.

## "`doctor` says my key has insecure permissions"

Private keys need to be readable only by you. Fix directly:
```bash
chmod 600 ~/.ssh/id_ed25519_<name>
```
On Windows, permissions are ACL-based rather than POSIX mode bits —
`doctor` detects an overly broad ACL (e.g. `Everyone` or `Authenticated
Users` granted access) and prints the exact `icacls` command to restrict
the key to your own account instead.

`doctor` reports this but doesn't fix it (RF-22's `doctor --fix` is
explicitly out of MVP scope — see PRD §26).

## "GitHub says the repository was not found" but I know it exists

GitHub returns the exact same error for "doesn't exist" and "you don't
have access" — deliberately, so private repos can't be enumerated by
guessing names. If Git Rodolfo doesn't recognize the error as coming from
one of its own `clone` attempts (e.g. you ran plain `git clone` instead of
`git rodolfo clone`), you'll see GitHub's raw message. Two things to check:
1. `git rodolfo current` (after a manual clone) or the account you passed
   to `git rodolfo clone --account <x>` — is it the account that should
   have access?
2. Ask whoever owns the repo whether your GitHub account is actually a
   collaborator/org member.

## "GitHub CLI registered my key on the wrong account"

`gh ssh-key add` always registers on whichever account `gh auth status`
currently shows as active — that's a `gh` behavior, not something Git
Rodolfo controls. Before registering, `git rodolfo account add`'s wizard
checks this and refuses with a clear mismatch message; but if you ran
`gh ssh-key add` directly, yourself, that check never ran. Fix: remove the
wrongly-registered key from https://github.com/settings/keys → the wrong
account, `gh auth switch` to the right one, then register again (via the
wizard, or `gh ssh-key add` directly once switched).

## "A network I'm on blocks outbound SSH (port 22)"

GitHub also accepts SSH on port 443 via `ssh.github.com`. Git Rodolfo's
MVP doesn't have a flag for this yet (tracked in PRD §26 as a post-MVP
improvement), but you can do it manually for a specific repository:
```bash
git config --local core.sshCommand "ssh -i '<key>' -o IdentitiesOnly=yes -p 443" \
  && git config --local url."ssh://git@ssh.github.com/".insteadOf "git@github.com:"
```
Run `git rodolfo current` afterward — it will report the repo as
"managed with inconsistency" since this bypasses Git Rodolfo's own
tracking; that's expected until §26's port-443 support lands.

## "My organization uses SSO and my key still gets rejected"

Some GitHub organizations require each SSH key to be separately authorized
for SSO, on top of just being registered on your account. `doctor` and the
account-verification flow both list this as a possible cause when
authentication fails, but can't authorize it for you — that has to happen
at https://github.com/settings/keys via GitHub's "Configure SSO" action on
the key.

## `account remove --delete-key` didn't actually delete the key

By design, Git Rodolfo refuses to delete a key file that's still claimed by
*another* registered account's fingerprint — since GitHub doesn't allow the
same public key on two accounts, this shouldn't come up in normal use, but
if you see "still used by another registered account," check
`git rodolfo accounts` for a stale duplicate profile first.

## Known test gaps

The automated suite (`go test ./...`) intentionally doesn't exercise a few
things that need a real network, a real terminal, or a real `gh` session:

- `internal/cli/prompt`'s raw-terminal arrow-key selector (only its
  non-interactive numbered fallback is covered — the fallback is what
  piped/test input always exercises, since a pipe is never a TTY).
- `SSHClient.TestConnectionWithKey`'s actual network call (its output-
  parsing logic, which is the part with real risk of a subtle bug, is
  covered separately against fixture text).
- `GitHubCLIAdapter`'s real `gh` calls (guarded, and skipped when `gh`
  isn't installed or authenticated; the account-matching logic they
  depend on is covered without `gh` at all).
- `SSHAgentAdapter.AddKey` against a real `ssh-agent` (it's real and runs,
  but only opt-in via `GIT_RODOLFO_TEST_SSH_AGENT=1`, since it mutates
  whatever `ssh-agent` the test machine has running).

`docs/TESTPLAN.md` is what covers these instead, with two real accounts.
