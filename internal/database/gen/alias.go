package gen

import (
	"sort"
	"strconv"
	"strings"
)

// UniqueAliases returns a table->alias map with collisions resolved by lengthening
// the shorter alias (or appending a numeric suffix when needed).
func UniqueAliases(tableNames []string) map[string]string {
	sorted := append([]string(nil), tableNames...)
	sort.Strings(sorted)

	aliases := make(map[string]string, len(sorted))
	used := make(map[string]string, len(sorted)) // alias -> table

	for _, table := range sorted {
		alias := ToAlias(table)
		if owner, ok := used[alias]; ok && owner != table {
			alias = lengthenAlias(table, alias, used)
		}
		used[alias] = table
		aliases[table] = alias
	}
	return aliases
}

func lengthenAlias(table, alias string, used map[string]string) string {
	parts := splitTableParts(table)
	candidate := alias
	// Prefer extending with more characters from segments.
	if len(parts) == 1 {
		name := parts[0]
		for n := len(candidate) + 1; n <= len(name); n++ {
			candidate = name[:n]
			if owner, ok := used[candidate]; !ok || owner == table {
				return candidate
			}
		}
	} else {
		for extra := 1; extra < 8; extra++ {
			var b []byte
			for _, p := range parts {
				if p == "" {
					continue
				}
				take := 1 + extra
				if take > len(p) {
					take = len(p)
				}
				b = append(b, p[:take]...)
			}
			candidate = string(b)
			if owner, ok := used[candidate]; !ok || owner == table {
				return candidate
			}
		}
	}
	for i := 2; ; i++ {
		candidate = alias + strconv.Itoa(i)
		if owner, ok := used[candidate]; !ok || owner == table {
			return candidate
		}
	}
}

func splitTableParts(tableName string) []string {
	parts := strings.Split(tableName, "_")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{tableName}
	}
	return out
}
