# reco

Repository Configurator: declare GitHub repository settings in YAML, review the diff, and apply it.

## Description

reco is a CLI for keeping settings of many GitHub repositories consistent.
You declare the desired state in a YAML file, and reco compares it with the current settings on GitHub, shows a change plan, and applies it after your approval.

Only settings you declare are managed. Omitted settings are left untouched.

### Supported settings

- General: allow auto-merge, automatically delete head branches
- GitHub Actions: allowed actions and reusable workflows, default `GITHUB_TOKEN` permissions, allowing GitHub Actions to create and approve pull requests

reco targets repositories owned by personal accounts on GitHub.com.
Organization-owned repositories and GitHub Enterprise Server are not supported.

## Usage

Create `reco.yaml`:

```yaml
repositories:
  - name: your-name/your-repo
    general:
      delete_branch_on_merge: true
      allow_auto_merge: true
    actions:
      workflow:
        default_workflow_permissions: read
        can_approve_pull_request_reviews: false
```

Set a token in `GH_TOKEN` or `GITHUB_TOKEN`, then run:

```shell
reco plan                        # show the change plan without updating
reco apply                       # show the plan, ask for approval, and apply
reco show --repo OWNER/REPO      # show current settings
```

Use `--config PATH` to specify another file, and `--repo OWNER/REPO` to limit `plan` / `apply` to one declared repository.

## Documentation

- [Requirements](docs/spec/requirements.md)
- [CLI specification](docs/spec/cli.md)
- [YAML configuration specification](docs/spec/configuration.md)

## Release notes

See [GitHub Releases][releases].

[releases]: https://github.com/tmknom/reco/releases
