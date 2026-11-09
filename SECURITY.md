# Security Policy

## Supported Versions

lurpicUI is a single-branch project: only the latest commit on `master` is
supported. There are no release branches to backport fixes to.

| Version | Supported |
|---------|-----------|
| master (latest commit) | yes |
| anything else | no |

## Reporting a Vulnerability

**Please do not open a public issue, discussion, or PR for security reports.**

Use GitHub's private vulnerability reporting on this repository:
**Security tab → "Report a vulnerability"** (private security advisory).
This reaches the maintainer directly and keeps the details out of the public
issue tracker until a fix is available.

Please include, when known:

- affected component and entry point (e.g. the `drive` local socket protocol,
  the `pakfs` asset filesystem parser, the Vulkan/software renderers, the
  `lurpic` CLI's project-file handling);
- a reproduction or proof of concept;
- impact assessment (local code execution, memory corruption, token/credential
  exposure, denial of service).

You can expect an acknowledgement within 7 days. If the report is accepted,
a fix lands on `master` behind the CI gate (`CI / workflow-lint`,
`CI / test` — see `devdocs/plans/host-ci-gate.md`), and the advisory is
published after the fix is merged.

## Scope notes

- lurpicUI is a local UI framework and CLI; its main attack surface is
  **malformed input files**: `.pak` asset archives, `lurpic.toml` project
  files, fonts, and drive-protocol frames on the local Unix socket (which is
  token-authenticated and mode-0700 by default).
- The CI workflows use a read-only `GITHUB_TOKEN` (`permissions: contents:
  read`), no secrets, and SHA-pinned actions; fork pull requests never
  receive secrets.

## Out of scope

- Vulnerabilities in third-party dependencies: report upstream, and we will
  pick up the fix via Dependabot (`gomod` updates are weekly).
- Social-engineering or physical-access attacks.
