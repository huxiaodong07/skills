# Authentication

`glab` handles authentication with its own CLI flow. The `skills` installer does not store, inject, or manage GitLab tokens for this skill.

## Login

```powershell
glab auth login
```

For a self-managed GitLab instance, specify the host if needed:

```powershell
glab auth login --hostname git.example.com
```

## Verify

```powershell
glab auth status
```

## Logout

```powershell
glab auth logout
```

## CI

In GitLab CI, use the official GitLab token mechanisms supported by `glab`, such as `CI_JOB_TOKEN` or project/group CI variables according to the command being run.

## Rules

- Do not write tokens into `SKILL.md`, `skill.json`, registry index files, or skill packages.
- Do not store tokens in Git remote URLs.
- Use the minimum token scope needed for the target operation.
- Do not print `PRIVATE-TOKEN`, `Authorization`, cookies, or token values in command output.
