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
skills search glab
skills info glab
skills install glab
skills plugin list
skills plugin install gitlab-tools
skills list
skills doctor
```

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

This repository is the CLI and aggregate registry. Individual public skills live in separate repositories, for example:

- https://github.com/huxiaodong07/glab
## Publish npm packages

```powershell
.\scripts\publish-npm.ps1 -Version 0.1.0
```

The public npm wrapper is `@xiaodonghu/skills`. The Windows binary package is `@xiaodonghu/skills-win32-x64`.




