---
description: "Company Go conventions, applied when editing Go files."
applyTo: "**/*.go"
---

> Replace this section with your company's real Go standards.

- Format with `gofmt`; keep functions small and single-purpose.
- Use table-driven tests and cover the error paths, not just the happy path.
- Wrap errors with context using `%w`; do not swallow errors.
- Prefer the standard library; justify any new dependency.
