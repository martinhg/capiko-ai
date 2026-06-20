# Security Policy

## Reporting a vulnerability

Please report security issues **privately** — do not open a public issue.

Use GitHub's [private vulnerability reporting](https://github.com/martinhg/capiko-ai/security/advisories/new)
for this repository. Include:

- A description of the issue and its impact.
- Steps to reproduce (and the output of `capiko-ai doctor` if relevant).
- The capiko-ai version (`capiko-ai version`) and your OS.

You'll get an acknowledgement, and we'll coordinate a fix and disclosure timeline with you.

## Supported versions

Security fixes target the **latest released version**. Upgrade to the newest release
(`brew upgrade`, the install script, or `go install ...@latest`) before reporting, in
case the issue is already fixed.

## Scope notes

capiko-ai writes configuration into your home directory (`~/.copilot/`, `~/.capiko/`)
and self-updates from GitHub releases. Reports about those surfaces — file handling,
the install/upgrade path, or the MCP wiring it writes — are especially in scope. capiko
never calls an LLM itself and ships no network service; it is a local CLI/TUI.
