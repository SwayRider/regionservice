# Code Review — `regionservice`

**Date:** 2026-08
**Scope:** Full review of `regionservice/` — geographic region lookup and border-crossing service for the SwayRider platform.
**Reviewed:** `cmd/regionservice/main.go`, `internal/bootstrap/bootstrap.go`, `internal/geodata/*` (manifest, reader, contour, border_crossing), `internal/index/*` (region, region_index, border_index, border_crossing, box, geoshape), `internal/server/*` (server, search_point, search_box, search_radius, find_crossing_locations, find_region_path, find_route_region_paths, ping), all test files, `protos/region/v1/region.proto`, `protos/health/v1/health.proto`, `Dockerfile`, `Makefile`, `env.example`, `local.env`, `.github/workflows/ci.yml`, the shared `swlib` app/security/logger machinery, the `rtreego` and `paulmach/orb` dependency sources, and the consuming `routerservice` + `grpcclients/regionclient` call sites.
**Verification performed:** `go build ./...`, `go vet ./...`, `go test ./... -count=1 -race`, and per-package coverage all run clean. No code changes were made.

---

## Summary

The service is cleanly structured and readable: a sensible four-layer split (`bootstrap → geodata → index → server`), a well-designed R-tree candidate-filtering approach with exact containment checks, a good `RegionQuerier`/`BorderQuerier` interface abstraction that makes handler testing easy, and a healthy set of handler-level tests (85% on `internal/server`, 100% on `internal/types`). The read-only-after-bootstrap indices are race-free, and everything builds/vets/tests cleanly.

However, there is one dominant problem and one crash bug that eclipse everything else:

1. **The service registers endpoint auth profiles but never installs the auth interceptor** — `main.go` passes `app.NoInterceptor, nil` to `NewGrpcConfig`, so the `region:query` / user-JWT enforcement that `server.go`'s `init()` declares (and the README promises, and that every other service in the repo actually enables) is **never evaluated**. Every endpoint is effectively public.
2. **A negative `limit` in `FindCrossingLocations` panics the process** (`make([]*BorderCrossingResult, 0, limit)` with a negative capacity), and `limit` is only defaulted when `== 0`, never validated for `< 0`. Combined with #1, any caller who can reach the service (it binds `[::]:8081` and `:8080`) can crash it remotely.

The rest of the findings are correctness/robustness issues in the spatial algorithms (incomplete box/circle intersection tests, incomplete antimeridian handling, a non-transitive crossing-ranking comparator, an unimplemented `/api/v1/health` endpoint) and hygiene items (dead code, unimplemented hash verification, Dockerfile hardening).

---

## Critical

### 1. ~~Endpoint authorization is declared but never enforced — all endpoints are public~~ - FIXED 2026-08-17
`cmd/regionservice/main.go:68`, `internal/server/server.go:47-56`

`server.go` registers every `RegionService` endpoint as `UserOrServiceEndpoint(..., []string{"region:query"})` and `init()` documents the intent clearly. But the gRPC server is constructed with **no interceptor**:

```go
grpcConfig := app.NewGrpcConfig(
    app.NoInterceptor, nil,          // main.go:68 — no AuthInterceptor
    ...
)
```

`swlib/app/grpc.go` only installs the auth check when the `AuthInterceptor` bit is set, and the enforcement itself lives entirely inside `swlib/grpc/interceptors/authinterceptor.go` (it looks up `security.GetEndpointProfile(info.FullMethod)` and calls `profile.Evaluate(...)`). With `NoInterceptor`, that lookup/evaluation **never runs**. Every other service in the repo enables it:

```go
// mailservice / searchservice / routerservice
app.AuthInterceptor | app.ClientInfoInterceptor, getPublicKeys, ...
```

So the `region:query` scope requirement, the user-JWT requirement, and the README's authorization table are all currently dead declarations. Any caller who can reach `:8081` (gRPC) or `:8080` (REST gateway) can:
- Enumerate every region and its geometry via `SearchPoint`/`SearchBox`/`SearchRadius`.
- Drive the expensive `FindCrossingLocations` and `FindRouteRegionPaths` computations (see #8) with arbitrary inputs, at no cost.

The `grpcclients/regionclient` and `routerservice` already send a `Bearer` token on every call — the token is simply never validated. This looks like an accidental omission (the `init()` block and README show clear intent), not a deliberate design choice.

Fix direction: pass `app.AuthInterceptor` (with a `JWTPublicKeysFn` from authservice, as the other services do) and keep the `Ping` endpoint `PublicEndpoint`. Add an auth-enforcement test (there is currently none — see Test-coverage gaps) so this can't silently regress.

---

## High

### 2. ~~Negative `limit` in `FindCrossingLocations` crashes the process (remote DoS)~~ - FIXED 2026-08-17
`internal/server/find_crossing_locations.go:99,152`, `internal/index/border_index.go:126`

The simple and advanced paths only default `limit` when it is zero:

```go
if req.Limit == 0 {
    req.Limit = 3
}
```

A negative `limit` passes straight through, and `FindCrossingLocations` then does:

```go
list := make([]*BorderCrossingResult, 0, limit)   // border_index.go:126
```

`make([]T, 0, -1)` panics with `runtime error: makeslice: cap out of range` (verified empirically). There is no `recover` interceptor installed, and gRPC does not recover handler panics by default, so the **entire process dies**. Because the endpoint is reachable over both REST (`POST /api/v1/region/find-crossing-locations`) and gRPC, and because of finding #1 there is no authentication, a single `{"limit": -1, ...}` request is a remote crash. Even with auth restored, a low-privileged user could still trigger it.

Fix direction: validate `req.Limit` (`<= 0` → default, or reject with `InvalidArgument`), and defensively guard the `make` against negative capacity regardless of caller.

### 3. ~~No input validation on coordinates, radius, or boxes~~ - FIXED 2026-08-17
`internal/server/search_point.go`, `search_box.go`, `search_radius.go`, `find_crossing_locations.go`

None of the search endpoints validate coordinate ranges or geometry sanity:
- **Lat/lon range** — no check that `lat ∈ [-90, 90]` or `lon ∈ [-180, 180]`. Out-of-range or `NaN`/`Inf` coordinates (trivially sendable over gRPC) flow into `rtreego.NewRect`, whose errors are silently discarded (`box, _ :=` at `region_index.go:71,186`). A malformed query box degrades to "R-tree matches everything, then the containment filter decides", which quietly returns empty or wrong results rather than an error.
- **`radius_km`** — never validated as positive. A negative radius produces an inverted box (`bottomLeft` ends up northeast of `topRight`), which makes `SearchByBox` compute `w < 0` and **incorrectly enter the antimeridian-split branch**, returning wrong/empty results.
- **Inverted boxes** — `SearchBox` with `topRight.lat < bottomLeft.lat` (`h < 0`) is not handled by the `w < 0` split; `NewRect` returns a `DistError` that is ignored, and the containment filter then can never match → silent empty result.

`FindRouteRegionPaths` is the one endpoint that *does* validate its numeric inputs (`width_km` must be positive) — the pattern exists and should be applied consistently.

Fix direction: add a shared coordinate/geometry validator (range checks, `NaN`/`Inf` rejection, `radius > 0`, `box` corner ordering) and propagate `rtreego.NewRect` errors instead of discarding them.

---

## Medium

### 4. ~~`containsOrIntersectsBox` / `containsOrIntersectsCircle` are incomplete intersection tests~~ - FIXED 2026-08-18
`internal/index/region_index.go:266,297`

Both helpers only test "a box/circle corner is inside the region" **or** "a region vertex is inside the box / within the radius":

```go
// containsOrIntersectsBox: any of the 4 box corners inside the polygon,
// OR any polygon vertex inside the box
// containsOrIntersectsCircle: center inside polygon OR any vertex within radius
```

A polygon edge that crosses a box edge (or a circle) with **no vertex inside** and **no corner inside** is missed — the classic edge-edge intersection case. For large country-sized contours this can silently drop regions that genuinely intersect the query. The comments acknowledge the approximation, but the result is a false-negative correctness gap in the two most-used search paths, and it's untested (see coverage gaps).

Fix direction: add proper segment/edge intersection checks (e.g., `orb`/`planar` segment-segment intersection) or use a robust polygon-rectangle / polygon-circle intersection, and add tests covering grazing/edge-crossing cases.

### 5. ~~Antimeridian handling is incomplete and internally inconsistent~~ - FIXED 2026-08-18
`internal/index/box.go`, `internal/index/region_index.go:124-236`

The code invests heavily in antimeridian machinery — `BoxLocation`, four-quadrant `Box`es, `SearchByBox`'s `w < 0` split — but the **final exact containment check never handles the wrap**:

- `SearchByPoint`/`SearchByBox`/`SearchByRadius` ultimately call `planar.MultiPolygonContains` on raw lon/lat coordinates. For a region whose geometry crosses ±180° (e.g., vertices at lon 179 and −179), planar ray-casting treats the polygon as spanning ~358°, so a query point on the "other side" of the dateline is reported as outside. The quadrant bboxes find the right candidates, but the exact check then rejects them.
- `BoxLocation.TransformPoint` (the intended ±360° longitude shift) is **never called anywhere in production** — it exists only in tests. The antimeridian story is half-built.
- `computeCorridorBox` (`find_route_region_paths.go:90`) computes `min/max` lon directly; a route crossing the dateline (e.g., waypoints at lon 179 and −179) yields a ~358°-wide corridor box, pulling nearly every region into the "allowed" set and blowing up the DFS in #8.

This is currently latent (the shipped regions are continental and don't cross the dateline), but the code clearly intends to support it. Fix direction: either complete the wrap handling (transform longitudes consistently before the planar containment check) or document the limitation and reject/flag dateline-crossing inputs rather than half-handling them.

### 6. ~~`FindCrossingLocations` ranking comparator is not a valid strict weak ordering~~ - FIXED 2026-08-18
`internal/index/border_index.go:100-125`

The `sort.Slice` comparator does not implement a consistent ordering:

```go
// If different road types, compare the *distance between the two crossings*
distMtrs := geo.Distance(cands[i].BorderCrossing.Location, cands[j].BorderCrossing.Location)
if distMtrs < roadTypeDelta {
    return roadFilter[...i...] < roadFilter[...j...]   // sort by road type
}
return cands[i].DistanceSquared < cands[j].DistanceSquared   // else by distance
```

Two issues:
- **Semantic mismatch with the documented intent.** The proto comment says `road_type_delta` is the distance within which crossings are "regarded as equally distant **from the line string**". The code instead compares the distance **between the two crossing points**, which is a different quantity and can reorder results counter to intent.
- **Non-transitivity.** The comparison between `i` and `j` depends on their mutual distance, so the ordering is not transitive; `sort.Slice` on a non-transitive comparator produces input-order-dependent, undefined ordering. The final ranking of crossings can vary between requests and is not reproducible.

Also: typos `thesshold` (line 116) and `closesForwardCrossing`/`closesBackwardCrossing` (`find_crossing_locations.go`) should be `threshold`/`closest...`.

Fix direction: rank by a well-defined key — sort primarily by (distance to the line segment, bucketed by `roadTypeDelta`) and secondarily by road-type priority — expressed as a proper comparator (or a stable multi-pass sort), and add tests that pin the expected ordering.

### 7. ~~`findCrossingDefinition` mutates the caller's slice and has fragile fallback semantics~~ - FIXED 2026-08-18
`internal/server/find_crossing_locations.go:243-260`

```go
slices.SortFunc(definitions, func(a, b *regionv1.BorderCrossingDefinition) int {
    return int(a.MaxBorderDistance - b.MaxBorderDistance)
})
for i := 1; i < len(definitions); i++ {
    if refDistance <= definitions[i].MaxBorderDistance {
        return definitions[i]
    }
}
return definitions[0]   // fallback = smallest max, unless a 0-max entry sorts first
```

- The input `definitions` slice is **sorted in place**, mutating the request. If a caller caches/reuses the definitions, they are permanently reordered.
- The fallback is `definitions[0]` after ascending sort. Per the proto, `max_border_distance = 0` is the "farthest away" fallback — correct only if such an entry exists. Without one (as in the unit test `TestFindCrossingDefinition`), the **farthest** crossing silently falls back to the **smallest** definition (`"exceeds all, fallback → max=50"`), which is almost certainly not the intended tuning for far crossings. This is a silent behavior trap for callers who omit the 0-max entry.

Fix direction: don't mutate the input (copy + sort, or pre-sort at config time), and make the fallback explicit (require/derive a 0-max definition) rather than incidental ordering.

### 8. ~~`FindRouteRegionPaths` can be expensive to the point of DoS, and corridor boxes over-approximate~~ - FIXED 2026-08-18
`internal/server/find_route_region_paths.go:31-86`, `internal/index/border_index.go:274-333`

The corridor search collects every core region intersecting each segment's expanded box, then runs an iterative DFS over the border-crossing graph enumerating **all acyclic paths** up to `maxLen = 2 × shortest`. The number of acyclic paths in a region graph can be exponential in the number of allowed regions; combined with the dateline over-approximation in #5 (which can admit nearly all regions), a single request can consume unbounded CPU. There is no result cap, no deadline, and the `ctx` parameter is never consulted (see #10), so a pathological request cannot be cancelled. With auth disabled (#1) this is remotely triggerable at no cost.

Fix direction: cap the number of returned paths, honor `ctx` cancellation, and tighten the corridor box computation (proper antimeridian handling and/or a smaller expansion).

### 9. ~~`HealthService.Check` is not implemented, and the README documents it~~ - FIXED 2026-08-18
`internal/server/server.go:97-112`, `internal/server/ping.go`, `protos/health/v1/health.proto`, `regionservice/README.md`

The proto defines both `Ping` and `Check`, and the README documents `GET /api/v1/health` (with `component` query param and `UNKNOWN/UP/DOWN` statuses). The server embeds `healthv1.UnimplementedHealthServiceServer` and implements **only `Ping`** — so `GET /api/v1/health` is routed by the gateway and then returns `codes.Unimplemented`. Documented functionality that doesn't exist. Either implement `Check` (even a trivial always-UP, matching the mailservice pattern) or remove it from the proto/README.

### 10. ~~`ctx` is threaded through every `BorderIndex` method but never used~~ - FIXED 2026-08-18
`internal/index/border_index.go:74,163,206,274,336`

`FindCrossingLocations`, `FindClosestCrossing`, `FindRegionPath`, `FindRouteRegionPaths`, and `findShortestAllowedPath` all take `ctx context.Context` and never read it. Long-running spatial queries and the DFS in #8 can never be cancelled or bounded by a deadline, tying up gRPC goroutines indefinitely. Either honor the context (check `ctx.Err()` in the loops) or drop the parameter.

---

## Low

### 11. ~~Dead code~~ - FIXED 2026-08-18
- `internal/index/border_crossing.go` — `SpatialborderCrossing` / `NewSpatialBorderCrossing` are unused; the R-tree-based crossing index they support is commented out in `border_index.go` (lines 41, 59, and the whole `CrossingLocations.add` block at 405-417). The commented-out `rtreego` import and `crossingLocations` field should be removed.
- `internal/index/box.go:79,212-266` — `BoxLocation.TransformPoint`, `LineSegment`, `NewLineSegment`, `Rect`, `NewRect`, `Contains`, `Within`, `Intersects`, `ContainsLineSegment`, `IntersectsLineSegment` are referenced only from tests, never from production code. Either wire them into the antimeridian logic (see #5) or delete them.

### 12. Manifest hash fields are documented but never verified
`internal/geodata/manifest.go`, `internal/geodata/contour.go`, `internal/geodata/border_crossing.go`

`ContourDesc.Hash`/`HashType` and `BorderCrossingDesc.Hash`/`HashType` are documented as "File content hash for verification", but no hash is ever computed or checked anywhere in the codebase. Either implement verification (protect against corrupted/mismatched geodata on the volume) or drop the fields and the misleading comments.

### 13. ~~`GetContour` returns `(nil, nil)` on unexpected GeoJSON → nil-pointer panic at bootstrap~~ - FIXED 2026-08-18
`internal/geodata/contour.go:18-58`

```go
switch v := data.(type) {
case geojson.FeatureCollection:
    features = &v
case geojson.Feature:
    features = &geojson.FeatureCollection{Features: []geojson.Feature{v}}
}
return   // bare Geometry or anything else → features stays nil, err stays nil
```

A contour file that parses as a bare geometry (or any other type) returns `(nil, nil)`; `parseFeature(nil)` then dereferences `gj.Features` and panics during startup instead of failing cleanly. Return an explicit error for unsupported GeoJSON types.

### 14. ~~No nil checks in bootstrap for missing contour descriptors~~ - FIXED 2026-08-18
`internal/bootstrap/bootstrap.go:27-38`

`region.Contour.Core` / `region.Contour.Extended` are dereferenced without nil checks. A manifest entry missing a contour (or missing `extended`) panics via nil dereference in `filepath.Join(r.geodataDir, contourDesc.RemoteFile)` rather than returning a clean error. Validate the manifest structure up front.

### 15. ~~`bootstrapFn` logs fatal *and* returns the error~~ - FIXED 2026-08-18
`cmd/regionservice/main.go:42-55`

```go
err := bootstrap.Bootstrap(reader, ri, bi)
if err != nil {
    lg.Fatalf("failed to bootstrap: %v", err)   // exits the process
}
return err                                        // dead code
```

`Fatalf` (via stdlib `log.Fatalf`) calls `os.Exit`, skipping deferred cleanup and graceful shutdown. Return the error and let the caller decide, or drop the dead `return`.

### 16. ~~Request mutation in the crossing handlers~~ - FIXED 2026-08-18
`internal/server/find_crossing_locations.go:95-103,150-156`

`findCrossingLocationsSimple`/`Advanced` write defaults back into the request (`req.Limit`, `cfg.RoadTypeDelta`, `cfg.DropDistance`, `cfg.RoadTypeOrder`). This is harmless for a single-use request but is a latent footgun if requests are ever pooled/reused, and it makes the handlers non-pure. Compute defaults into locals.

### 17. ~~Silent road-type fallback to `MOTORWAY`~~ - FIXED 2026-08-18
`internal/server/find_crossing_locations.go:133-139,197-203`

```go
RoadType: regionv1.RoadType(regionv1.RoadType_value[strings.ToUpper(item.BorderCrossing.RoadType.String())])
```

An empty or unrecognized road type indexes the `RoadType_value` map to `0`, silently reporting the crossing as `MOTORWAY`. An out-of-range enum in `road_type_order` likewise becomes `""` in the filter. Guard against unknown values rather than defaulting to the highest-priority road type.

### 18. ~~Non-deterministic path results~~ - FIXED 2026-08-18
`internal/index/border_index.go:206-264,274-333`

`FindRegionPath` and the DFS in `FindRouteRegionPaths` iterate Go maps (`for region, list := range endpoints`, `for neighbor := range i.regionCrossings[last]`). Among multiple equal-length paths, `FindRegionPath` returns a **random** one; path ordering in `FindRouteRegionPaths` results is random. Deterministic ordering (sort by region name) would make behavior reproducible and testable.

### 19. ~~Dockerfile hardening~~ - FIXED 2026-08-18
`Dockerfile`

- `FROM golang:latest` — unpinned, mutable base tag; builds are not reproducible.
- `COPY . .` with **no `.dockerignore`** — the build context ships `.git/`, `local.env` (present in the working tree, machine-specific paths), `.DS_Store`, and the `REST/` remnants. Add a `.dockerignore`.
- `CGO_ENABLED=1` with cross-gcc toolchains for both arches, though the code has no cgo imports — unnecessary complexity; `CGO_ENABLED=0` would produce static binaries and drop the whole cross-compiler block.
- No `HEALTHCHECK` (the service has a health RPC that could drive it).

### 20. ~~Minor documentation/config drifts~~ - FIXED 2026-08-18
- `regionservice/README.md` — the "Authorization" table omits `FindRouteRegionPaths` (registered in `server.go`), and the "API Reference" section states "All endpoints are public and require no authentication", directly contradicting the auth table above it. (Note: the "public" statement happens to match current reality given #1, but the two sections can't both be the intended contract.)
- `env.example` omits `LOG_LEVEL`, which the README lists as configurable.
- `local.env` hardcodes a machine-specific `GEODATA_DIR` (`/Users/maartenheremans/...`); it's gitignored so not a leak, but it's a trap if the file is ever shared.

### 21. ~~`SearchByPoint`'s query box is offset, not centered~~ - FIXED 2026-08-18
`internal/index/region_index.go:67-73`

```go
box, _ := rtreego.NewRect(rtreego.Point{p[0], p[1]}, []float64{pointSize, pointSize})
```

`rtreego.NewRect` treats the point as the **min corner**, so the box spans `[p, p + 0.0001°]` (northeast of the point) rather than being centered on it. No false negatives (the point itself is included), so this is cosmetic, but a centered box (`point.ToRect(pointSize/2)`) would match the intent and avoid the slight NE bias.

---

## Positive observations

- **Clean, layered architecture** — `bootstrap → geodata → index → server` with clear single responsibilities; each layer is small and readable.
- **Sound index design** — R-trees are used for cheap candidate filtering and exact `planar` containment for the final answer; the quadrant-bbox split for antimeridian *boxes* (`SearchByBox`'s `w < 0` handling) is thoughtful.
- **Good interface abstraction** — `RegionQuerier`/`BorderQuerier` make the handlers fully mockable; the handler tests are genuinely useful and cover validation, defaulting, and response mapping well (85% on `internal/server`).
- **Read-only-after-bootstrap indices** — no mutation after startup, so concurrent reads are safe; `go test -race` is clean.
- **Fail-fast bootstrap** — a missing/corrupt geodata manifest aborts startup loudly rather than serving empty indexes.
- **Centralized, documented auth intent** — all endpoint access levels are declared in one `init()` block, which makes the intent obvious (even though enforcement is currently missing — see #1).
- **`FindRouteRegionPaths` validates its numeric inputs** (`width_km > 0`, `>= 2 waypoints`) — the validation pattern exists and can be extended to the other endpoints.
- **Tests pass, race-clean, vet-clean** — `go test ./...`, `go test -race`, and `go vet` all green.

---

## Test-coverage gaps

Measured with `go test -cover`:

| Package | Coverage | Notes |
| ------- | -------- | ----- |
| `internal/types` | 100% | Road-type parsing — good. |
| `internal/server` | 85% | Good handler coverage; mocks only. |
| `internal/index` | 49.3% | **The core spatial logic is 0% covered** (see below). |
| `internal/geodata` | 0% | Manifest / GeoJSON contour / CSV border-crossing parsing entirely untested. |
| `internal/bootstrap` | 0% | Untested. |
| `cmd/regionservice` | 0% | No startup/bootstrap/auth test. |

Specific gaps:

- **`internal/index/region_index.go` is 0% covered** — `SearchByPoint`, `SearchByBox`, `SearchByRadius`, `Add`, `parseFeature`, `containsOrIntersectsBox`, `containsOrIntersectsCircle`, `geomPolygonToOrbPolygon`, and the whole `Region`/`GeoShape`/`SpatialRegion` machinery have **no tests at all**. The index tests only cover `box.go` helpers and `border_index.go`. The heart of the service is unverified; #3, #4, and #5 would all have been caught by even basic point/box/radius tests.
- **No auth-enforcement test** — nothing asserts that an unauthenticated call to a `region:query` endpoint is rejected. Finding #1 would be caught immediately by a test that boots the gRPC server with the configured interceptor and calls a protected endpoint without a token.
- **No negative/edge input tests** — no tests for negative `limit` (#2), inverted boxes, negative `radius_km`, `NaN`/out-of-range coordinates (#3).
- **No geodata tests** — no tests for malformed GeoJSON (bare geometry, empty file) or manifest-shaped-mismatch panics (#13, #14), and no test that the documented hash verification (which doesn't exist) is enforced (#12).
- **No antimeridian-region test** — nothing exercises a region whose geometry crosses ±180° (#5).
- **`findCrossingDefinition` test documents the fragile fallback** rather than pinning intended semantics (#7); the crossing-ranking comparator has no ordering assertions (#6).
- **No corridor edge-case tests** — `computeCorridorBox` dateline behavior and DFS blow-up (#5, #8) are untested.

---

## Recommended fix order

1. **#1 (critical)** — enable `app.AuthInterceptor` (with a real `JWTPublicKeysFn` from authservice) so the declared `region:query`/user-JWT enforcement actually runs; add an auth-enforcement test.
2. **#2 (high)** — validate `limit` (reject `< 0`; default `<= 0`) and guard the `make` capacity; add a regression test. This is the crash bug.
3. **#3 (high)** — add shared coordinate/radius/box validation and stop discarding `rtreego.NewRect` errors.
4. **#4 / #5 (medium)** — fix the incomplete box/circle intersection tests and either complete or explicitly bound the antimeridian story (including `computeCorridorBox`), with tests.
5. **#6 / #7 (medium)** — make crossing ranking a well-defined ordering (and stop mutating the request/definitions slices).
6. **#8 / #10 (medium)** — cap `FindRouteRegionPaths` output, honor `ctx` cancellation, and add deadlines.
7. **#9 (medium)** — implement (or remove) `HealthService.Check`; fix the README auth table/contradiction.
8. **#11–#21 (low)** — remove dead code, implement or drop hash verification, harden `GetContour`/bootstrap nil handling, add `.dockerignore`, pin the base image, and clean up the remaining nits.

Items #1 and #2 are the priority: #1 is a security regression against documented intent, and #2 is a trivially reachable remote crash.
