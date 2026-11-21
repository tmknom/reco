# reco

Repository Configurator: declare GitHub repository settings in YAML, review the diff, and apply it.

## Description

reco is a CLI for keeping settings of many GitHub repositories consistent.
You declare the desired state in a YAML file, and reco compares it with the current settings on GitHub, shows a change plan, and applies it after your approval.

Only settings you declare are managed. Omitted settings are left untouched.

### Supported settings

- General: allow auto-merge, automatically delete head branches
- GitHub Actions: enabling or disabling GitHub Actions, allowed actions and reusable workflows, requiring actions to be pinned to a full-length commit SHA, default `GITHUB_TOKEN` permissions, allowing GitHub Actions to create and approve pull requests, access to actions and reusable workflows from other repositories (private repositories only)
- Environments: existence of declared environments and their deployment branch restriction (`all`, `protected_branches`, or `selected` with its branch and tag patterns); `apply` creates missing environments, changes the restriction, adds and removes `selected` patterns one at a time, and never touches environments that are not declared
- Security: enabling or disabling Dependabot alerts and Dependabot security updates; `plan` shows that disabling Dependabot alerts also disables the dependency graph and Dependabot security updates on GitHub
- Secret scanning: enabling or disabling Secret scanning and repository push protection on public repositories; `plan` shows that disabling Secret scanning also disables push protection on GitHub

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
- [YAML configuration specification](docs/spec/configuration.md)

## Release notes

See [GitHub Releases][releases].

[releases]: https://github.com/tmknom/reco/releases
