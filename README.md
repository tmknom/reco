# reco

Repository Configurator: declare GitHub repository settings in YAML, review the diff, and apply it.

## Description

reco is a CLI for keeping settings of many GitHub repositories consistent.
You declare the desired state in a YAML file, and reco compares it with the current settings on GitHub, shows a change plan, and applies it after your approval.

Only settings you declare are managed. Omitted settings are left untouched.

### Supported settings

- General: allow auto-merge, automatically delete head branches
- GitHub Actions: enabling or disabling GitHub Actions, allowed actions and reusable workflows, default `GITHUB_TOKEN` permissions, allowing GitHub Actions to create and approve pull requests

reco targets repositories owned by personal accounts on GitHub.com.
Organization-owned repositories and GitHub Enterprise Server are not supported.

## Installation

Download an archive from [GitHub Releases][releases], or use the container image `ghcr.io/tmknom/reco` (no `latest` tag).
See [Installation](docs/user-guide/installation.md) (in Japanese) for verifying the downloads and preparing a token.

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
