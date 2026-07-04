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
skills search demo
skills info demo
skills install demo
skills list
skills doctor
```

## Packaging commands

```powershell
skills pack path\to\skill --out dist
skills publish path\to\skill --gitlab-url https://git.example.com --project-id 123 --dist dist --token-env GITLAB_API_PAT
```

`skills` does not manage tool tokens. Authentication is handled by each installed skill or its CLI.

## Publish npm packages

```powershell
.\scripts\publish-npm.ps1 -Version 0.1.0
```

The public npm wrapper is `@xiaodonghu/skills`. The Windows binary package is `@xiaodonghu/skills-win32-x64`.


