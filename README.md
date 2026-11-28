# reco

Repository Configurator: declare GitHub repository settings in YAML, review the diff, and apply it.

## Description

reco is a CLI for keeping settings of many GitHub repositories consistent.
You declare the desired state in a YAML file, and reco compares it with the current settings on GitHub, shows a change plan, and applies it after your approval.

Only settings you declare are managed. Omitted settings are left untouched.

### Supported settings

- General: allow auto-merge, automatically delete head branches, allowed merge methods (squash merge, merge commit, rebase merge; not all three can be disabled), always suggesting updating pull request branches, default commit message title and body for squash merges and merge commits, repository features (Issues, Projects, Wiki), requiring sign-off on web-based commits
- GitHub Actions: enabling or disabling GitHub Actions, allowed actions and reusable workflows, requiring actions to be pinned to a full-length commit SHA, default `GITHUB_TOKEN` permissions, allowing GitHub Actions to create and approve pull requests, access to actions and reusable workflows from other repositories (private repositories only), approval required before running workflows from fork pull requests (public repositories only), whether workflows from fork pull requests run and receive write tokens, secrets, and variables (private repositories only; the change plan notes when write tokens or secrets and variables start being sent to those workflows), how many days artifacts, logs, checks, workflow runs, and commit statuses are kept (up to the limit GitHub allows for each repository), and the customization of the OIDC token subject claim (the change plan notes that a change may stop matching trust conditions in external cloud providers)
- Repository Variables: declared variables and their values; `apply` creates missing variables, updates values that differ, and deletes variables declared with `state: absent` after showing the value to be lost, and variables that are not declared (including ones whose lines were removed) are never touched
- Environments: existence of declared environments and their deployment branch restriction (`all`, `protected_branches`, or `selected` with its branch and tag patterns); `apply` creates missing environments, changes the restriction, adds and removes `selected` patterns one at a time, and deletes environments declared with `state: absent` after showing the protection settings to be lost and the environment variables and secrets that GitHub deletes with them (secrets by name only, as their values cannot be read); environments that are not declared (including ones whose entries were removed) are never touched
- Environment required reviewers: the declared set of users who can approve deployment jobs for an environment; `apply` sets, changes, or removes them, and `plan` shows that removing every reviewer also discards Prevent self-review on GitHub
- Environment Prevent self-review and wait timer: whether the user who triggered a deployment job may approve it, and how many minutes the job waits before it runs; `plan` reports Prevent self-review as not applicable when the environment would have no reviewers, which GitHub requires for it
- Environment Variables: declared variables of each declared environment and their values; `apply` creates the environment first if it is missing, then creates missing variables, updates values that differ, and deletes variables declared with `state: absent`, and variables that are not declared are never touched
- Security: enabling or disabling Dependabot alerts and Dependabot security updates; `plan` shows that disabling Dependabot alerts also disables the dependency graph and Dependabot security updates on GitHub
- Secret scanning: enabling or disabling Secret scanning and repository push protection on public repositories; `plan` shows that disabling Secret scanning also disables push protection on GitHub
- Private vulnerability reporting: enabling or disabling private vulnerability reporting on public repositories
- Rulesets: the whole of each declared branch ruleset on public repositories (enforcement, target branches, bypass actors, and the `pull_request`, `deletion`, `required_status_checks`, and `non_fast_forward` rules); `apply` creates a declared ruleset after checking that no ruleset with that name exists yet, and replaces an existing one with the declaration or deletes one declared with `state: absent` after checking that the whole ruleset is still what `plan` showed (including the rules and bypass actors that would be removed), and rulesets that are not declared are never touched

reco targets repositories owned by personal accounts on GitHub.com.
Organization-owned repositories and GitHub Enterprise Server are not supported.

## Installation

Download a binary from [GitHub Releases][releases], or use the container image `ghcr.io/tmknom/reco` (no `latest` tag).
See [Installation](docs/user-guide/installation.md), [Authentication](docs/user-guide/authentication.md), and [Verification](docs/user-guide/verification.md) (in Japanese) for installing, preparing a token, and verifying the downloads.

## Usage

Create `reco.yaml`:

```yaml
repositories:
  - name: your-name/your-repo
    general:
      delete_branch_on_merge: true
```

Set a token in `GH_TOKEN` (typically `export GH_TOKEN=$(gh auth token)`), then run:

```shell
reco show --repo your-name/your-repo   # show current settings
reco plan                              # show the change plan without updating
reco apply                             # show the plan, ask for approval, and apply
```

For a step-by-step first run, see [Getting Started](docs/user-guide/getting-started.md) (in Japanese).

## Documentation

- [User Guide](docs/user-guide/README.md) (in Japanese): what reco solves, how to install and use it, how to read its output, and what to do when something goes wrong

For developers:

- [Requirements](docs/spec/requirements.md)
- [CLI specification](docs/spec/cli.md)
- [Output specification](docs/spec/output.md)
- [YAML configuration specification](docs/spec/configuration.md)

## Release notes

See [GitHub Releases][releases].

[releases]: https://github.com/tmknom/reco/releases
