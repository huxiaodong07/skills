# skills-cli

Internal skill package manager.

## MVP commands

```powershell
skills registry add ciqtek http://172.16.30.151:8099/MR/skills/skills-index/-/raw/main/registry-index.json
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
