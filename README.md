# ai-skills

A package manager for private AI Skills. Skills are packaged as OCI Artifacts and distributed through standard container registries — no separate infrastructure required.

## Supported Registries

| Registry | Reference format |
|---|---|
| GitHub Container Registry | `ghcr.io/<org>/<namespace>/<skill>:<tag>` |
| Azure Container Registry | `<name>.azurecr.io/<namespace>/<skill>:<tag>` |
| GitLab Container Registry | `registry.gitlab.com/<group>/<namespace>/<skill>:<tag>` |
| Any OCI-compliant registry | `<host>/<namespace>/<skill>:<tag>` |

Docker does not need to be installed. `ai-skills` communicates directly with registries over HTTPS using [oras-go](https://oras.land).

---

## Installation

```bash
go install github.com/DavidW475/ai-skills@latest
```

Or download a pre-built binary from the [Releases](../../releases) page.

---

## Concepts

### Skill package

A skill is a directory containing at minimum:

```
my-skill/
├── skill.yaml   # manifest: name, version, description, author, tags
└── SKILL.md     # Copilot skill instructions (YAML frontmatter + markdown)
```

**`skill.yaml`**
```yaml
name: my-skill
version: "1.0.0"
description: Helps write SQL queries following team conventions.
author: myorg
tags:
  - sql
  - database
```

**`SKILL.md`** — standard VS Code Copilot skill format:
```markdown
---
name: my-skill
description: 'Helps write SQL queries following team conventions.'
---

# My Skill

Describe what the skill does and how Copilot should use it.
```

### Global install model

Skills are installed **globally** for the current user:

| Path | Purpose |
|---|---|
| `~/.ai-skills/sources` | Registry namespaces to search |
| `~/.ai-skills/installed.yaml` | Index of installed skills |
| `~/.ai-skills/config.yaml` | Optional config (e.g. custom install dir) |
| `~/.agent/skills/<name>/` | Default skill install location |

VS Code Copilot discovers skills from `~/.agent/skills/` automatically.  
The install directory can be overridden in `~/.ai-skills/config.yaml`:

```yaml
skills_dir: ~/my/custom/path
```

---

## Usage

### 1. Authenticate

```bash
ai-skills login ghcr.io
# Username: myuser
# Password: ghp_...   (Personal Access Token)
```

Use a **Personal Access Token** (PAT) or **Deploy Token** — registries do not support TOTP for API access.

### 2. Add a source registry

```bash
ai-skills source add ghcr.io/myorg/skills
ai-skills source add registry.gitlab.com/mygroup/skills
```

### 3. Browse available skills

```bash
ai-skills search
# or
ai-skills available
```

Lists all skills found in configured sources together with their latest version and installed status.

### 4. Create a new skill

```bash
ai-skills init my-skill --version 1.0.0
# Edit my-skill/SKILL.md and my-skill/skill.yaml
```

### 5. Publish a skill

```bash
ai-skills publish ./my-skill ghcr.io/myorg/skills/my-skill:v1.0.0
# → Published ghcr.io/myorg/skills/my-skill:v1.0.0
#   digest: sha256:abc123...
```

### 6. Install a skill

```bash
ai-skills install ansible           # latest semver tag
ai-skills install ansible@v1.0.0   # exact version (v-prefix optional)
ai-skills install ansible@1.0.0    # same as above
```

Version resolution order:
1. Lists all tags from each configured source
2. Picks the highest semver tag (e.g. `v1.2.0` beats `v1.1.0`)
3. For explicit versions, tries both `:v1.0.0` and `:1.0.0`

### 7. Update skills

```bash
ai-skills update              # update all installed skills
ai-skills update ansible      # update a specific skill
```

### 8. List and remove skills

```bash
ai-skills list                             # list installed skills
ai-skills versions ansible                 # list all remote versions for a skill
ai-skills uninstall ansible                # remove skill files + index entry
ai-skills uninstall --keep-files ansible   # remove only from index
```

### 9. Web UI

```bash
ai-skills ui
# UI running at http://localhost:8080
# Press Ctrl+C to stop.
```

The web dashboard provides three tabs:

| Tab | Description |
|---|---|
| **Installed** | View installed skills, check for updates, update individually or all at once |
| **Install** | Browse available skills with version selector and one-click install/upgrade/downgrade |
| **Sources** | Add and remove registry sources |

Use `--addr` to bind to a different address:

```bash
ai-skills ui --addr localhost:9090
```

---

## CI/CD

### GitHub Actions

```yaml
- name: Install AI Skills
  run: |
    go install github.com/DavidW475/ai-skills@latest
    echo "${{ secrets.GITHUB_TOKEN }}" | ai-skills login ghcr.io -u ${{ github.actor }}
- name: Install skills
  run: |
    ai-skills source add ghcr.io/myorg/skills
    ai-skills install ansible
```

### GitLab CI

```yaml
install-skills:
  script:
    - go install github.com/DavidW475/ai-skills@latest
    - echo "$CI_REGISTRY_PASSWORD" | ai-skills login registry.gitlab.com -u $CI_REGISTRY_USER
    - ai-skills source add registry.gitlab.com/mygroup/skills
    - ai-skills install ansible
```

---

## Command Reference

| Command | Description |
|---|---|
| `ai-skills init <name>` | Scaffold a new skill directory |
| `ai-skills publish <dir> <ref>` | Pack and push a skill to an OCI registry |
| `ai-skills install <name>[@version]` | Install a skill globally |
| `ai-skills update [name...]` | Update installed skills to their latest version |
| `ai-skills uninstall <name> [name...]` | Remove installed skills |
| `ai-skills list` | List all locally installed skills |
| `ai-skills versions <name>` | List all remote versions for a skill |
| `ai-skills search` | List all skills available in configured sources |
| `ai-skills source add <registry>` | Add a registry namespace to sources |
| `ai-skills source remove <registry>` | Remove a registry namespace |
| `ai-skills source list` | Show all configured sources |
| `ai-skills login <registry>` | Store registry credentials |
| `ai-skills ui` | Start the local web dashboard |

All commands accept `--plain-http` for local/self-hosted registries without TLS.

---

## License

See [LICENSE](LICENSE).
