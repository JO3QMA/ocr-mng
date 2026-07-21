# Review prevention (implement before PR)

Catch recurring OCR/PR findings at implement time. Design-time depth (cross-cutting): user skill `grill-review-ready`. Use this list before declaring a feature done.

## Store / SQL

- [ ] Claim-next / “has active” + insert in one transaction (no TOCTOU between check and create)
- [ ] Migrations and multi-step copies use a transaction
- [ ] `sql.NullString` / nullable columns: check `.Valid` before `.String`
- [ ] Delete/Update: `RowsAffected == 0` is not-found, not silent success
- [ ] `json.Marshal` / scan errors checked; do not ignore Marshal failures into empty payloads
- [ ] Avoid N+1 (e.g. list providers then per-provider model queries) — batch or join at store layer
- [ ] Test-only helpers stay in `_test.go` (not public `Store` methods)

## Engine / review run

- [ ] Panic recovery path updates the same artifacts as the normal failure path (e.g. PR snapshot)
- [ ] TriggerKind and similar enums are named constants, not scattered string literals
- [ ] Dispatch / DB work uses a bounded context (not bare `context.Background()` under a long-held lock)
- [ ] Cancellable work uses `context` (not bare `time.After` that leaks the worker)

## Web / forms / i18n

- [ ] Sensitive POST forms: CSRF posture matches project decision (do not add new unauthenticated state-changing POSTs casually)
- [ ] Settings save must not overwrite newly submitted fields with stale `prev.*` values
- [ ] Nav labels match page titles; protocol/error allowlists stay in sync with the `ocr` package
- [ ] URL parse: scheme-less input is not treated as a valid host (relative path pitfall)

## General Go

- [ ] After `bufio.Scanner`, check `sc.Err()`
- [ ] Reject `\n` / `\r` inside persisted paths or config lines (TrimSpace is not enough)
- [ ] No `ponytail:` markers in committed code (TODO/FIXME or delete)

Diff the change set against this checklist before PR. Failures → fix now.
