# Code Review — 2026-08-19

Follow-up security audit of the full current codebase (not diff-based), cross-checked against [`review/CODE_REVIEW_2026-08.md`](CODE_REVIEW_2026-08.md). See [`Docs/REVIEW.md`](../../Docs/REVIEW.md) for how findings in this file are tracked.

**Verification of prior review:** all previously-FIXED Critical/High items were spot-checked against current code and remain fixed, no regressions found — notably #1 (missing auth interceptor: `AuthInterceptor`/`RateLimitInterceptor` are wired in `cmd/regionservice/main.go`, and every data RPC still requires the `region:query` scope), #2 (negative-limit panic: clamped in `internal/index/border_index.go` and both handlers in `find_crossing_locations.go`), #3 (coordinate/box/radius validation: `internal/server/validate.go` rejects NaN/Inf and out-of-range values), and #8/#10 (unbounded DFS / missing context cancellation: `maxRouteRegionPaths`/`maxRouteRegionPathSteps` caps and `ctx.Err()` checks are in place). Item #12 (manifest content hashes unverified) remains open as previously tracked — not duplicated here.

No SQL/query-injection surface exists (no database — the service is a pure in-memory R-tree/GeoJSON index loaded from a local manifest at startup), and no secrets were found hardcoded in `env.example` or `local.env` (gitignored).

### 1. Dockerfile runs the service as root

The runtime stage (`debian:bookworm-slim`) never adds/switches to a non-root `USER`. Failure scenario: any future RCE-class bug (e.g. a geodata-parsing vulnerability, or a dependency CVE) grants root inside the container rather than a restricted user — a defense-in-depth gap, not itself exploitable today. Severity: Low.

### 2. No upper bound on `radius_km` or query-box area

`validateRadiusKm` only requires the value to be `> 0` and finite; a request like `radius_km: 1e8` is accepted. Not a meaningful DoS vector given the architecture: the indexed dataset is small, fixed-size, and fully in-memory (continental-scale region polygons, no per-query disk/DB work), so worst case is "return every region" — bounded, cheap work. Recorded as accepted-risk rather than a gap. Severity: Info.

### 3. Internal-only gating relies solely on JWT/scope enforcement, not network isolation

The service binds `:8080`/`:8081` with no mTLS or network-policy restriction to the gateway. This matches intended design now that #1 is fixed (auth via JWT+scope, not network segmentation) — flagging only because if the ports are ever reachable directly (e.g. a misconfigured Kubernetes Service), the JWT+scope check is the sole barrier. That check is correctly wired today. Severity: Info.
