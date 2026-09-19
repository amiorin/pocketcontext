# PocketContext

PocketContext adds SQL reads to PocketBase. Applications such as DealContext own their collections, migrations, hooks, and workflow documentation.

All application writes go through PocketBase's standard REST API. Never give agent SQL requests a write connection. Do not expose auth collections, hidden fields, system tables, SQLite metadata, or arbitrary extensions through the SQL endpoint.

The SQL endpoint has a shared-workspace permission model. Every account in the configured auth collection can read every configured column. PocketBase collection API rules do not filter these SQL results.

Build with Go 1.27 or later, CGO enabled, and a C compiler. Run `make test` and `make build` after code changes. Security tests belong in `internal/sqlread` and HTTP integration tests in `internal/server`.

Keep documentation literal and concise. Do not add a CRM frontend or CRM-specific code here.
