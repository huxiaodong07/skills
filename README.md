# skills-cli

General-purpose skill package manager.

## Install from npm

```powershell
npm install -g @xiaodonghu/skills
skills --help
```

Fresh installs use the public default registry:

```text
https://raw.githubusercontent.com/huxiaodong07/skills/main/registry/registry-index.json
```

## MVP commands

```powershell
skills registry list
skills search
skills search glab
skills info glab
skills install glab
skills info gitlab-tools
skills install gitlab-tools
skills list
skills doctor
```

## Proxy configuration

```powershell
skills config get proxy
skills config set proxy system
skills config set proxy none
skills config set proxy http://127.0.0.1:7890
```

`system` reads `HTTP_PROXY`, `HTTPS_PROXY`, and `NO_PROXY`. Environment variables `SKILLS_PROXY_MODE` and `SKILLS_PROXY_URL` override the saved config for the current process.

When an installed skill contains a `bin/` directory, commands are copied into the unified user command directory:

```text
~/hxdSkills/bin
```

Only this directory is added to the user PATH.

## Packaging commands

```powershell
skills pack path\to\skill --out dist
skills publish path\to\skill --gitlab-url https://git.example.com --project-id 123 --dist dist --token-env GITLAB_API_PAT
```

`skills` does not manage tool tokens. Authentication is handled by each installed skill or its CLI.


## Skill repositories

This repository is the CLI and aggregate registry. A skill source repository can be private, while installable skill packages are hosted by this registry under `registry/packages/`.

Example:

- source repository: https://github.com/huxiaodong07/glab
- install package: `registry/packages/glab/0.1.0/glab-0.1.0.skillpack.tar.gz`

Public third-party skill repositories can also be referenced directly with `archive_url` and `source_path`, without copying their package into this registry.
## Publish npm packages

```powershell
.\scripts\publish-npm.ps1 -Version 0.1.0
```

The public npm wrapper is `@xiaodonghu/skills`. The Windows binary package is `@xiaodonghu/skills-win32-x64`.




