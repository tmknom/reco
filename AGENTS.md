# AGENTS.md

reco (Repository Configurator) declares GitHub repository settings in YAML, shows the diff against the current settings, and applies it after approval. Built as a Go CLI.

## Rules

- Read `llms.txt` at task start, then read the documents the task needs
- Write commit messages and PR descriptions in English
- Safety first: when safety conflicts with anything else, choose the safe option; never relax the invariants in [Requirements](docs/spec/requirements.md) on your own judgment, and leave that decision to a human
- When adding a requirement or comparing design options, check it against the relevant principles in [Constitution](docs/CONSTITUTION.md)
- Test first: write a failing test before the code
- Keep code and docs in sync within the same change
