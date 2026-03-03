# jf skill - AI Agent Skill Management

Manage AI agent skills, rules, and commands stored in Artifactory generic repositories.

## Concepts

A **skill pack** is a collection of files that configure an AI agent (e.g., Cursor). It lives in a `.cursor/` directory and may contain:


| Content Type        | Location                   | Format                             |
| ------------------- | -------------------------- | ---------------------------------- |
| Rules               | `.cursor/rules/`           | `.mdc` files                       |
| Skills              | `.cursor/skills/`          | `SKILL.md` files in subdirectories |
| Commands            | `.cursor/commands/`        | `.md` files                        |
| Global Instructions | `AGENTS.md` (project root) | Markdown                           |
| Metadata            | `.cursor/manifest.yaml`    | YAML                               |


## Cursor Skills on GitHub Today

A typical Cursor skills repository on GitHub looks like this:

```
my-cursor-skills/
├── .cursor/
│   ├── rules/
│   │   ├── coding/
│   │   │   └── style-guide.mdc
│   │   └── review/
│   │       └── pr-checklist.mdc
│   ├── skills/
│   │   ├── debugging/
│   │   │   └── SKILL.md         
│   │   └── refactoring/
│   │       └── SKILL.md
│   └── commands/
│       └── lint-fix.md
├── AGENTS.md
└── README.md
```

Skills are loaded from `.cursor/skills/` (project-level) or `~/.cursor/skills/` (global). The agent invokes them automatically by context or manually via `/skill-name`.

Real-world examples:

- [jfrog/ecomatrix-code-manifesto](https://github.com/jfrog/ecomatrix-code-manifesto) — coding standards and rules
- [JFROG/ask-blender-buddy](https://github.jfrog.info/JFROG/ask-blender-buddy) — skills, rules, commands, and AGENTS.md for incident analysis
- [JFROG/jfrog-ai-kit](https://github.jfrog.info/JFROG/jfrog-ai-kit) — JFrog AI agent skill kit

The limitation with GitHub-hosted skills is manual — you clone a repo, copy files into `.cursor/`, and repeat for updates. There's no versioning, dependency tracking, or security scanning. That's what `jf skill` solves.

---

## Why Artifactory for AI Agent Skills?

Public skill registries have proven the need for centralized skill distribution. However, public registries and GitHub alone fall short for enterprise use:


| Concern                 | GitHub / Public Registry      | Artifactory                                    |
| ----------------------- | ----------------------------- | ---------------------------------------------- |
| Discovery               | Keyword search across repos   | search, categories, versioned packs            |
| Install UX              | Clone + manual copy           | `jf skill install <ref> --repo=<repo>`         |
| Versioning              | Git tags (manual)             | Semver with `@version` and latest resolution   |
| Dependency management   | None                          | Manifest-based batch install                   |
| Security scanning       | Limited (Dependabot for code) | Xray scanning on every upload                  |
| Access control          | Repo-level permissions        | Fine-grained repo/path permissions, audit logs |
| Air-gapped environments | Not supported                 | Full on-prem / air-gapped support              |


Artifactory acts as the **controlled intermediary** — teams can pull vetted skills from public sources, scan them with Xray, and serve only approved packs to developers. This is the same proxy-and-curate pattern used for npm, Maven, PyPI, and Docker.

### The ClawHavoc Supply Chain Attack

The importance of this approach was underscored by **ClawHavoc** (Feb 2026) — a coordinated supply chain attack on a public AI skill registry:

- **335 malicious skills** traced to a single threat actor ([Repello AI analysis](https://repello.ai/blog/clawhavoc-supply-chain-attack))
- **~12% of the registry** was compromised, affecting ~300,000 users ([Koi Security audit](https://openclawconsult.com/lab/openclaw-clawhavoc-supply-chain))
- Primary payload: **Atomic Stealer (AMOS)** — stole API keys, SSH keys, crypto wallets, browser passwords
- Three attack techniques were used:
  1. **Prompt injection via SKILL.md** — adversarial instructions embedded in skill files caused agents to leak environment variables to attacker-controlled servers
  2. **Reverse shell via hidden scripts** — skills triggered malicious shell scripts during normal agent operations
  3. **Token exfiltration via CVE-2026-25253** (CVSS 8.8) — one-click RCE through the agent control UI ([Antiy Labs analysis](https://www.antiy.net/p/clawhavoc-analysis-of-large-scale-poisoning-campaign-targeting-the-openclaw-skill-market-for-ai-agents/))

These techniques apply to **any AI agent platform** (Cursor, Claude Code, Windsurf) that loads third-party skill files as trusted instructions.

**How Artifactory mitigates this:**

- **Xray scanning** catches known malicious patterns on upload
- **Curation policies** — only approved packs reach developers
- **Audit trail** — full traceability of who published what and when
- **Private/air-gapped** — skills never touch a public network

---

## Repository Layout

Packs are stored in Artifactory generic repositories:

```
skills-local/                              # Artifactory generic repository
├── jfrog/                                 # namespace
│   ├── blender-buddy/                     # pack name
│   │   ├── 1.0.0/                         # version
│   │   │   ├── manifest.yaml
│   │   │   ├── AGENTS.md
│   │   │   ├── rules/
│   │   │   │   ├── coding/
│   │   │   │   │   └── style-guide.mdc
│   │   │   │   └── review/
│   │   │   │       └── pr-checklist.mdc
│   │   │   ├── skills/
│   │   │   │   └── incident-analysis/
│   │   │   │       └── SKILL.md
│   │   │   └── commands/
│   │   │       └── run-diagnostics.md
│   │   └── 2.0.0/
│   │       ├── manifest.yaml
│   │       └── ...
│   └── ai-kit/                            # another pack
│       └── 1.0.0/
│           ├── manifest.yaml
│           └── ...
└── acme/                                  # another namespace
    └── security-pack/
        └── 1.0.0/
            ├── manifest.yaml
            └── ...
```

---

## Commands

### jf skill publish

Upload a local skill pack to Artifactory.

```
jf skill publish <namespace>/<pack>@<version> --repo=<repo-name> [--server-id=<id>] [--skill-path=<path>]
```

**Flags:**


| Flag           | Required | Description                                                  |
| -------------- | -------- | ------------------------------------------------------------ |
| `--repo`       | Yes      | Artifactory repository name                                  |
| `--server-id`  | No       | JFrog CLI server configuration ID                            |
| `--skill-path` | No       | Path to `.cursor/` directory (defaults to `.cursor/` in cwd) |


**Examples:**

```bash
# Publish from current directory
jf skill publish myteam/coding-standards@1.0.0 --repo=skills-local

# Publish from a specific path
jf skill publish myteam/coding-standards@1.0.0 --repo=skills-local --skill-path=/path/to/project/.cursor

# Publish to a specific server
jf skill publish myteam/coding-standards@1.0.0 --repo=skills-local --server-id=my-server
```

**Behavior:**

- Scans `.cursor/` for rules, skills, and commands
- Generates/updates `manifest.yaml` with accurate content counts
- Checks for `AGENTS.md` in the parent directory and uploads it if found
- Warns if the version already exists (files are overwritten)

---

### jf skill install

Download and install skill packs from Artifactory. Supports two modes.

#### Mode 1: Single Pack Install

```
jf skill install <namespace>/<pack>@<version> --repo=<repo-name> [flags]
```

**Flags:**


| Flag              | Required | Description                                |
| ----------------- | -------- | ------------------------------------------ |
| `--repo`          | Yes      | Artifactory repository name                |
| `--server-id`     | No       | JFrog CLI server configuration ID          |
| `--rules-only`    | No       | Install only rules                         |
| `--skills-only`   | No       | Install only skills                        |
| `--commands-only` | No       | Install only commands                      |
| `--category`      | No       | Install only a specific category subfolder |
| `--no-agents`     | No       | Skip installing AGENTS.md                  |


**Examples:**

```bash
# Install a full pack
jf skill install jfrog/blender-buddy@2.0.0 --repo=skills-local

# Install latest version (omit @version)
jf skill install jfrog/blender-buddy --repo=skills-local

# Install only rules
jf skill install jfrog/blender-buddy@2.0.0 --repo=skills-local --rules-only

# Install rules from a specific category
jf skill install jfrog/blender-buddy@2.0.0 --repo=skills-local --rules-only --category=coding

# Install without AGENTS.md
jf skill install jfrog/blender-buddy@2.0.0 --repo=skills-local --no-agents
```

**Behavior:**

- Downloads files into `.cursor/` in the current directory
- Replaces existing content type folders (clean install) in single pack mode
- Downloads `AGENTS.md` to project root (unless `--no-agents` or filter flags are set)
- Adds/updates the installed pack in `.cursor/manifest.yaml` dependencies (upserts by namespace/pack)

#### Mode 2: Manifest Install (Dependencies)

```
jf skill install --repo=<repo-name> [--server-id=<id>]
```

When run with no arguments, reads `.cursor/manifest.yaml` and installs all listed dependencies.

**Why dependencies?** Not every project owns all its skills. Skills follow the same ownership boundaries as code — teams that own a library should own its skills too.

**Example 1: Cross-team skill sharing**

Different teams in an organization publish and maintain their own skill packs independently. Consumer projects compose them via `manifest.yaml`:

```
┌─────────────────────────────────────────────────────────────┐
│                    Artifactory (skills-local)                │
│                                                              │
│  ┌──────────────────────┐  ┌──────────────────────────────┐ │
│  │ acme/security-rules  │  │ acme/devops-commands         │ │
│  │ @1.0.0               │  │ @2.0.0                       │ │
│  │                      │  │                              │ │
│  │ rules/               │  │ commands/                    │ │
│  │   xss-prevention.mdc │  │   deploy.md                  │ │
│  │   sql-injection.mdc  │  │   rollback.md                │ │
│  │                      │  │                              │ │
│  │ Published by:        │  │ Published by:                │ │
│  │ Security Team        │  │ DevOps Team                  │ │
│  └──────────┬───────────┘  └───────────────┬──────────────┘ │
│             │                              │                 │
└─────────────┼──────────────────────────────┼─────────────────┘
              │       jf skill install       │
              │         --repo=skills-local  │
              ▼                              ▼
┌─────────────────────────────────────────────────────────────┐
│  my-project/.cursor/manifest.yaml                            │
│                                                              │
│  dependencies:                                               │
│    packages:                                                 │
│      - ref: acme/security-rules@1.0.0    ◄── from Security  │
│    commands:                                                 │
│      - ref: acme/devops-commands@2.0.0   ◄── from DevOps    │
│                                                              │
│  + project's own skills in .cursor/skills/  (local)          │
└──────────────────────────────────────────────────────────────┘
```

Each team versions and publishes their pack independently. Consumer projects declare what they need and `jf skill install` pulls it all in.

**Example 2: Tightly-coupled vs independent libraries**

Consider JFrog CLI — it has sub-projects like `jfrog-cli-artifactory` (tightly coupled, can't exist on its own) and dependencies like `build-info-go` and `jfrog-client-go` (independent libraries with their own teams and release cycles):

```
┌──────────────────────────────────────────────────────────┐
│  jfrog-cli (GitHub repo)                                  │
│                                                           │
│  ┌─────────────────────────────────────────────────────┐  │
│  │  jfrog-cli own skills                               │  │
│  │  skills/cli-commands/SKILL.md                       │  │
│  │  rules/cli-conventions.mdc                          │  │
│  └─────────────────────────────────────────────────────┘  │
│                                                           │
│  ┌─────────────────────────────────────────────────────┐  │
│  │  jfrog-cli-artifactory (tightly coupled, embedded)  │  │
│  │  skills/artifactory-ops/SKILL.md                    │  │
│  │  rules/rt-conventions.mdc                           │  │
│  │                                                     │  │
│  │  Can't exist independently — lives in same repo     │  │
│  └─────────────────────────────────────────────────────┘  │
│                                                           │
│  manifest.yaml                                            │
│    dependencies:                                          │
│      packages:                                            │
│        - ref: jfrog/build-info-go-skills@1.0.0            │
│        - ref: jfrog/client-go-skills@3.0.0                │
└──────────────────────┬───────────────────────┬────────────┘
                       │                       │
                       │  jf skill install     │
                       │  --repo=skills-local  │
                       ▼                       ▼
          ┌──────────────────┐    ┌──────────────────┐
          │ build-info-go    │    │ jfrog-client-go  │
          │ skills @1.0.0    │    │ skills @3.0.0    │
          │                  │    │                  │
          │ Independent repo │    │ Independent repo │
          │ Own team & cycle │    │ Own team & cycle │
          └──────────────────┘    └──────────────────┘
```

Tightly-coupled skills stay in the same repo. Independent libraries publish their own skill packs. The consuming project imports them as dependencies — keeping skill ownership aligned with code ownership.

**Example:**

```bash
# Install all dependencies from manifest
jf skill install --repo=skills-local
```

**Behavior:**

- Reads the `dependencies` section from `.cursor/manifest.yaml`
- Installs each dependency sequentially
- Overlays files from multiple packs (does not wipe between dependencies)

---

## manifest.yaml

The manifest serves dual purposes: pack metadata (for publish) and dependency declaration (for install).

### Full Schema

```yaml
name: my-project
namespace: myteam
version: 1.0.0
agent: cursor
description: Optional description
contents:
  skills: 2
  rules: 3
  commands: 1
  agents_md: true
dependencies:
  packages:
    - ref: jfrog/blender-buddy@2.0.0
    - ref: acme/security-pack@1.0.0
  rules:
    - ref: acme/coding-rules@1.0.0
      category: coding
  skills:
    - ref: acme/debugging-skills@3.0.0
  commands:
    - ref: acme/ci-commands@1.2.0
```

### Dependencies Section


| Section    | Installs                                          | Notes                                          |
| ---------- | ------------------------------------------------- | ---------------------------------------------- |
| `packages` | Full pack (rules + skills + commands + AGENTS.md) | Equivalent to running `jf skill install <ref>` |
| `rules`    | Rules only                                        | Supports optional `category` filter            |
| `skills`   | Skills only                                       |                                                |
| `commands` | Commands only                                     |                                                |


Each entry requires a `ref` field in the format `<namespace>/<pack>@<version>`. The `--repo` flag provides the repository for all dependencies.

---

## End-to-End Workflow

```bash
# 1. Author a skill pack locally
mkdir -p .cursor/rules .cursor/skills/my-skill .cursor/commands
echo "---\ndescription: Style guide\n---\nUse consistent naming." > .cursor/rules/style.mdc
echo "# My Skill\nDo something useful." > .cursor/skills/my-skill/SKILL.md
echo "# Lint\nRun the linter." > .cursor/commands/lint.md

# 2. Publish to Artifactory
jf skill publish myteam/my-pack@1.0.0 --repo=skills-local --server-id=my-server

# 3. Install in another project
cd /path/to/other-project
jf skill install myteam/my-pack@1.0.0 --repo=skills-local --server-id=my-server

# 4. Or declare as a dependency and batch install
cat > .cursor/manifest.yaml << 'EOF'
dependencies:
  packages:
    - ref: myteam/my-pack@1.0.0
  rules:
    - ref: acme/security-rules@2.0.0
EOF

jf skill install --repo=skills-local --server-id=my-server
```

