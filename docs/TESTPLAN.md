# Manual end-to-end test plan

This is the suite RNF-09 calls for: "a documented manual end-to-end suite,
with two real test accounts, covering the acceptance criteria that require
a real provider (§22)." It exists because the automated test suite
(`go test ./...`) deliberately never talks to real GitHub or a real
`ssh-agent` session shared with other tools — that's what makes it safe to
run in CI. The 23 criteria in PRD §22 that *do* require a real provider are
covered here instead.

## Prerequisites

- Two real GitHub accounts you control (referred to below as **Personal**
  and **Work**). At least one should have access to a private repository
  the other does **not** have access to — that's what exercises CU-07.
- macOS or Linux, Git ≥ 2.30, OpenSSH, and (optionally) GitHub CLI (`gh`)
  installed.
- A freshly built binary: `make build`, then either put `./git-rodolfo` on
  your `PATH` or invoke it as `./git-rodolfo` throughout.
- An isolated config for the run, so this doesn't touch your real
  `~/.config/git-rodolfo`:

  ```bash
  export XDG_CONFIG_HOME="$(mktemp -d)"
  ```

Work through the steps below in order — later ones assume earlier ones
succeeded. Check off each §22 criterion as its corresponding step passes.

## 1. Install and invoke (§22 #1, #2)

- [ ] `git-rodolfo --version` works.
- [ ] `git rodolfo --version` (via Git's subcommand mechanism) prints the
      identical output. `diff <(git-rodolfo accounts) <(git rodolfo accounts)`
      should be empty.

## 2. Register both accounts (§22 #3, #4, #5, #6)

For **Personal**:

```bash
git-rodolfo account add
```
Walk the interactive wizard: generate a new key, no passphrase needed for
this pass. Confirm it ends with a real `✓ SSH key accepted` /
`✓ GitHub account detected: <personal-username>` block — this is the RF-13
network round-trip against real GitHub, and the only way to genuinely
validate it.

- [ ] Account registers successfully. (#3)
- [ ] `git-rodolfo account show personal` shows `verified` with a recent
      timestamp.

For **Work**, repeat, but this time:
- [ ] Add a passphrase when prompted, and choose "Add it to ssh-agent now."
      Confirm you're asked for the passphrase once by `ssh-add` — never by
      Git Rodolfo — and that a second command (`git-rodolfo account show
      work`) doesn't ask again. (#6)
- [ ] Repeat `account add` for a **third** profile pointing at the same key
      file you generated for Personal. Confirm it's rejected with the
      "already used by the account" message, not a generic error. (#4)
- [ ] `ssh-keygen -t ed25519 -f /tmp/existing-key -N ""`, then
      `account add --key /tmp/existing-key`: confirm Git Rodolfo accepts an
      existing key file. (#5)

## 3. GitHub CLI registration (RF-14)

Only if `gh` is installed and authenticated:
- [ ] Run `gh auth switch` to the **Personal** session, then run
      `account add --register-key-with-gh` for an account configured as
      **Work**. Confirm you get the "GitHub CLI is authenticated as X, but
      this account is Y" message, not a silent registration on the wrong
      account.
- [ ] Switch `gh` to the matching session and retry: confirm
      `gh ssh-key add` succeeds and the key shows up at
      https://github.com/settings/keys.

## 4. Edit and remove (§22 #8, #9, #10, #11)

- [ ] `git-rodolfo account edit personal`, change the commit email, confirm
      the before/after summary and the "repositories configured with Git
      Rodolfo" warning. (#8)
- [ ] `git-rodolfo account remove personal`, decline the key deletion:
      confirm the profile is gone (`accounts` no longer lists it) but the
      key files are untouched on disk. (#9)
- [ ] Re-add Personal (you'll need it for step 6), then remove it again,
      this time confirming key deletion: confirm both `.pub` and private
      key files are gone. (#10)
- [ ] Attempt `account remove work --delete-key --non-interactive` without
      `--yes`: confirm it refuses. Attempt `--yes` alone (no
      `--delete-key`): confirm the key survives. Only `--yes --delete-key`
      together deletes it. (#11)

Re-add Personal once more before continuing — later steps need it.

## 5. Clone with the correct account, with another key loaded (§22 #12) — the single most important test

This is the test that validates §12.2's whole architecture decision. If it
fails, that decision needs to be reopened — it is not a "fix it later" bug.

```bash
ssh-add -l                      # confirm both Personal's and Work's keys are loaded
git-rodolfo clone git@github.com:<work-org>/<a-private-work-repo>.git --account work
```

- [ ] The clone succeeds using the **Work** identity, even though
      **Personal**'s key was offered first by `ssh-agent`. (#12)
- [ ] Inside the clone: `git config --local --list` shows `core.sshcommand`
      pinned to Work's key, and the remote is still the plain
      `git@github.com:...` URL (not rewritten).

## 6. `gh` and IDEs still work inside a managed repo (§12.2 validation criterion #2)

Inside the repo cloned above:
- [ ] `gh repo view` works without any extra flags or config.
- [ ] `gh pr create` (against a throwaway branch/PR you're prepared to
      close) works without extra flags.

## 7. Clone with the wrong account (§22 #13, CU-07)

```bash
git-rodolfo clone git@github.com:<work-org>/<a-private-work-repo>.git --account personal
```

- [ ] Output explains it may be a missing-repo **or** access problem,
      states which account authenticated, states who the repo belongs to,
      suggests the correct registered account if one matches the org name,
      and includes GitHub's original error text at the end. Not the raw
      `ERROR: Repository not found.` alone.

## 8. `use` / `current` / `use --clear` (§22 #14, #15, #16, #17)

- [ ] The repo cloned in step 5 has the right `user.name`/`user.email` and
      the canonical remote. (#14)
- [ ] `git init /tmp/scratch-repo && cd /tmp/scratch-repo && git-rodolfo use work`:
      succeeds with no remote, and says so plainly. (#15)
- [ ] `git-rodolfo use --clear` in that repo, then
      `git config --local --list`: no `user.name`, `user.email`,
      `rodolfo.account`, or `core.sshcommand` entries remain. (#16)
- [ ] Hand-edit `user.email` in the step-5 repo to something else, then
      `git-rodolfo current`: reports the drift with an actionable `git
      rodolfo use <account>` suggestion. Then `cd /tmp && git-rodolfo current`
      (a non-repo directory): confirm the clear "not a Git repository"
      message, not a crash. (#17)

## 9. `doctor` (§22 #18)

- [ ] Temporarily `chmod 644` one account's private key: `doctor` flags it
      and suggests `chmod 600 <path>`.
- [ ] Temporarily rename a key file: `doctor` reports it missing with the
      exact expected path.
- [ ] `export GIT_SSH_COMMAND="ssh -i /some/other/key"` and run `doctor`
      inside the step-5 repo: confirm validation #8 catches it, then
      `unset GIT_SSH_COMMAND`.
- [ ] Temporarily point one account's key at a file that doesn't
      authenticate (e.g. a key never added to GitHub): confirm `doctor`
      reports the authentication failure with the two possible causes from
      §13.10's example.
- [ ] Register the same key fingerprint on two profiles (via manual
      `config.json` edit, since `account add` itself blocks this): confirm
      `doctor` reports the shared key.

## 10. Secrets and `~/.ssh/config` (§22 #19)

- [ ] `grep -r "BEGIN.*PRIVATE KEY" "$XDG_CONFIG_HOME/git-rodolfo"` matches
      nothing.
- [ ] `diff <(cat ~/.ssh/config 2>/dev/null) <(cat ~/.ssh/config.bak-before-testing 2>/dev/null)`
      (a copy taken before this whole test run) is empty — the file is
      untouched byte for byte, per §12.2's validation criterion #3.

## 11. Non-interactive mode (§22 #20)

- [ ] Re-run the core flows (`account add`, `account edit`, `account
      remove`, `clone`, `use`) with `--non-interactive` and the
      appropriate flags, confirming each either completes without any
      prompt or fails with a clear message naming the missing flag.

## 12. Platform coverage (§22 #21)

- [ ] Run this entire plan once on macOS.
- [ ] Run at least steps 1, 2, 5, 7, 8, 9 on a Linux distribution (a
      throwaway container or VM is fine, since step 5 needs real network
      access to GitHub).

## 13. Automated coverage exists (§22 #22)

- [ ] `go test ./... -race` passes with no skips other than the
      documented ones (see `docs/TROUBLESHOOTING.md`'s "Known test gaps"
      section).

## 14. Errors are actionable (§22 #23)

- [ ] Look back over every error message hit during this run — CU-07,
      duplicate key, unsupported host, insecure permissions, missing key,
      auth failure. Each one should have told you what to run next, not
      just what went wrong.

## Cleanup

```bash
git-rodolfo account remove personal --yes --delete-key --non-interactive
git-rodolfo account remove work --yes --delete-key --non-interactive
rm -rf "$XDG_CONFIG_HOME" /tmp/scratch-repo /tmp/existing-key /tmp/existing-key.pub
```
Also remove the throwaway SSH keys from https://github.com/settings/keys
on both accounts, and close/delete any test PR created in step 6.
