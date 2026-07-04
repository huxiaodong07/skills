# @xiaodonghu/skills

Skills manager CLI.

## Install

```bash
npm install -g @xiaodonghu/skills
```

## Usage

```bash
skills --help
skills registry add personal <registry-index-url>
skills search
skills search glab
skills install glab
skills install gitlab-tools
```

The npm package only installs the `skills` manager. Skill packages are still resolved from configured skills registries.

## Proxy

```bash
skills config set proxy system
skills config set proxy none
skills config set proxy http://127.0.0.1:7890
```

