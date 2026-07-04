# glab

GitLab workflow skill for using the `glab` CLI.

This skill describes safe and repeatable GitLab workflows. It does not bundle credentials and it does not manage GitLab tokens. Authentication is handled by `glab` itself.

## Authentication

Use the official `glab` authentication flow for your GitLab host.

```powershell
glab auth login
glab auth status
```

For self-managed GitLab instances, pass your host when needed:

```powershell
glab auth login --hostname git.example.com
```

See `docs/authentication.md`.

## Common Commands

```powershell
glab auth status
glab repo list
glab mr list
glab issue list
glab pipeline list
```

## Safety Rules

- Do not store tokens in `SKILL.md`, `skill.json`, registry index files, or skill packages.
- Do not put tokens in Git remote URLs.
- Let `glab` handle authentication, host configuration, and token storage using its own supported mechanisms.
- Review destructive commands before creating, updating, deleting, transferring, merging, or publishing GitLab resources.

## Troubleshooting

- Run `glab auth status` to inspect authentication state.
- Verify the target GitLab host is reachable.
- Use the minimum token scope needed for the operation.
