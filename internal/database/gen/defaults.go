package gen

import (
	"regexp"
	"strings"

	"github.com/ilaziness/app-tpl/internal/constant"
)

var (
	pgCastRE    = regexp.MustCompile(`(?i)^(.*?)(::[a-zA-Z_][\w.]*(?:\([^)]*\))?)+$`)
	numericRE   = regexp.MustCompile(`^-?\d+(\.\d+)?$`)
	quotedStrRE = regexp.MustCompile(`^'([^']|'')*'$`)
	currentTSRE = regexp.MustCompile(`(?i)^current_timestamp(?:\(\s*\d*\s*\))?$`)
	pgNowRE     = regexp.MustCompile(`(?i)^now\(\s*\)$`)
)

// NormalizeDefault returns a Bun default tag value for the driver, or empty to omit.
func NormalizeDefault(driver, raw string, goType string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "NULL") {
		return ""
	}

	baseType := strings.TrimPrefix(goType, "*")
	if baseType == "bool" {
		return ""
	}

	switch driver {
	case constant.DriverMySQL:
		return normalizeMySQLDefault(raw)
	case constant.DriverPostgres, constant.DriverPostgreSQL:
		return normalizePostgresDefault(raw)
	case constant.DriverSQLite, constant.DriverSQLite3:
		return normalizeSQLiteDefault(raw)
	default:
		return ""
	}
}

func normalizeMySQLDefault(raw string) string {
	if numericRE.MatchString(raw) {
		return raw
	}
	if quotedStrRE.MatchString(raw) {
		return raw
	}
	if currentTSRE.MatchString(raw) {
		return "current_timestamp"
	}
	return ""
}

func normalizePostgresDefault(raw string) string {
	if strings.Contains(strings.ToLower(raw), "nextval(") {
		return ""
	}
	// Strip PostgreSQL type casts from the end: 'active'::text, 1::integer
	cleaned := raw
	for {
		m := pgCastRE.FindStringSubmatch(cleaned)
		if m == nil {
			break
		}
		next := strings.TrimSpace(m[1])
		if next == cleaned {
			break
		}
		cleaned = next
	}
	if pgNowRE.MatchString(cleaned) {
		return "now()"
	}
	if currentTSRE.MatchString(cleaned) {
		return "current_timestamp"
	}
	if numericRE.MatchString(cleaned) {
		return cleaned
	}
	if quotedStrRE.MatchString(cleaned) {
		return cleaned
	}
	return ""
}

func normalizeSQLiteDefault(raw string) string {
	if numericRE.MatchString(raw) {
		return raw
	}
	if quotedStrRE.MatchString(raw) {
		return raw
	}
	if currentTSRE.MatchString(raw) {
		return "current_timestamp"
	}
	return ""
}

// SanitizeComment flattens whitespace and strips comment terminators.
func SanitizeComment(comment string) string {
	comment = strings.ReplaceAll(comment, "*/", "")
	comment = strings.Join(strings.Fields(comment), " ")
	return strings.TrimSpace(comment)
}
