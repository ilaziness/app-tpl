package router

const (
	PathHealth    = "/health"
	PathReadiness = "/readiness"
	PathLiveness  = "/liveness"
	PathVersion   = "/version"

	PathClientV1   = "/api/client/v1"
	PathClientV2   = "/api/client/v2"
	PathAdminV1    = "/api/admin/v1"
	PathAdminV2    = "/api/admin/v2"
	PathInternalV1 = "/api/internal/v1"

	PathClientV1Users   = PathClientV1 + "/users"
	PathClientV2Users   = PathClientV2 + "/users"
	PathAdminV1Users    = PathAdminV1 + "/users"
	PathInternalV1Users = PathInternalV1 + "/users"

	PathSwagger = "/swagger/*any"
)

// SystemPaths are public system endpoints that should skip auth, rate limits, and observability noise.
var SystemPaths = []string{PathHealth, PathReadiness, PathLiveness, PathVersion}

// RateLimitSkipPaths returns Gin FullPath patterns excluded from rate limiting.
func RateLimitSkipPaths() []string {
	paths := make([]string, 0, len(SystemPaths)+1)
	paths = append(paths, SystemPaths...)
	paths = append(paths, PathSwagger)
	return paths
}

// DefaultJWTSkipPaths returns paths that skip JWT authentication by default.
// Client APIs stay public (optional Bearer still parsed into claims).
// Internal APIs authenticate via X-Internal-Service-Key (or RequireAuth fallback).
// Admin business routes still require JWT via RequireAuth when jwt.secret is set.
func DefaultJWTSkipPaths() []string {
	paths := make([]string, 0, len(SystemPaths)+6)
	paths = append(paths, SystemPaths...)
	paths = append(paths,
		PathSwagger,
		PathClientV1+"/*",
		PathClientV2+"/*",
		PathInternalV1+"/*",
		PathAdminV1+"/auth/login",
		PathClientV1+"/auth/login",
	)
	return paths
}

// MergeJWTSkipPaths merges config jwt.skip_paths onto DefaultJWTSkipPaths.
// Defaults are always included; config may add extra paths. Duplicates are dropped.
func MergeJWTSkipPaths(configPaths []string) []string {
	base := DefaultJWTSkipPaths()
	if len(configPaths) == 0 {
		return base
	}
	seen := make(map[string]struct{}, len(base)+len(configPaths))
	out := make([]string, 0, len(base)+len(configPaths))
	for _, p := range base {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	for _, p := range configPaths {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}
