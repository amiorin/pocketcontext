# PocketContext

PocketContext adds SQL reads to PocketBase. Applications such as DealContext own their collections, migrations, hooks, and workflow documentation.

All application writes go through PocketBase's standard REST API. Never give agent SQL requests a write connection. Do not expose auth collections, hidden fields, system tables, SQLite metadata, or arbitrary extensions through the SQL endpoint.

The default SQL endpoint has a shared-workspace permission model. Every account in the configured auth collection can read every configured column. Optional filtered snapshots apply application-owned SQL row filters before agent queries run. PocketBase collection API rules do not filter either mode's SQL results.

Filtered snapshots must bind identity from authentication, export all tables from one source read transaction, and fail without results on incomplete exports. Keep policy tables out of the query database and schema response. Agent queries must use a separate read-only snapshot connection with no source database attached. Preserve explicit column allowlists, export limits, cleanup, and isolation between requests. Keep application-specific policies in application configuration and tests.

Build with Go 1.27 or later, CGO enabled, and a C compiler. Run `make test` and `make build` after code changes. Security tests belong in `internal/sqlread` and HTTP integration tests in `internal/server`.

Keep documentation literal and concise. Do not add a CRM frontend or CRM-specific code here.
