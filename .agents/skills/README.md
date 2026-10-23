# Skills

Procedures an assistant follows when working on this package.

They live in `.agents/skills/<name>/SKILL.md`, which is the path the coding
assistants read from — Cursor, Codex, Cline, Copilot, Gemini CLI, Amp, OpenCode,
Warp, Zed and the rest all look there. It is one directory rather than a file
per vendor, so a skill written once is read by whatever this package is being
written with.

Each file opens with frontmatter carrying a `name` and a `description`. The
`name` has to equal the directory name exactly, or the skill is not loaded. The
description is what a tool reads to decide whether the skill is relevant, so it
names the situation you are in rather than the topic it covers.

| skill | when it fires |
| --- | --- |
| `playwright-release` | the gates, the conformance run in three engines, the manifest, a dependency, a version, a tag |
| `playwright-vault-notes` | writing the note, the gap or the journal entry, when this checkout sits inside the Arandu Obsidian vault |
| `playwright-package` | installing and registering this driver **in an application's tests** |

The last one has a different audience, and that is on purpose: it travels with
the package so that an assistant working in somebody else's project has the
install, the one line in `tests/TestCase.go` and the environment in front of
it. Its frontmatter says so, under `metadata`, with `audience: app`, which is
what `aru skills:sync` reads to copy it into an application whose `go.mod`
requires this package.

`playwright-vault-notes` applies only when `MOC-arandu.md`,
`plans/cmd/audit-vault/` and `45-modules/` are beside this checkout. Outside the
vault it is inert, and says so first.

## Adding your own

A skill in this directory is yours and travels with the repository. Keep it a
procedure rather than a description: a file that says "read the documentation"
never changes what anybody does.
