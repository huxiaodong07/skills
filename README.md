# skills-cli

Internal skill package manager.

## Install from npm

```powershell
npm install -g @hxd/skills
skills --help
```

## MVP commands

```powershell
skills registry add ciqtek http://172.16.30.151:8099/api/v4/projects/144/repository/files/registry-index.json/raw?ref=main
skills search glab
skills info glab
skills install glab
skills list
skills doctor
```

## Packaging commands

```powershell
skills pack D:\ToolManage\skills\glab --out dist
skills publish D:\ToolManage\skills\glab --gitlab-url http://172.16.30.151:8099 --project-id 142 --dist dist --token-env GITLAB_API_PAT
```

`skills` does not manage tool tokens. Authentication is handled by each installed skill or its CLI.

## Publish npm packages

```powershell
.\scripts\publish-npm.ps1 -Version 0.1.0
```

The public npm wrapper is `@hxd/skills`. The Windows binary package is `@hxd/skills-win32-x64`.

