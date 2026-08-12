package cache

import "strings"

// KeyGenerator generates structured cache keys from a prefix and parts.
type KeyGenerator interface {
	// Generate builds a cache key. For example: Generate("user", "123", "profile")
	// returns "user:123:profile".
	Generate(prefix string, parts ...string) string
}

// DefaultKeyGenerator joins the prefix and parts with ":" as the separator.
type DefaultKeyGenerator struct{}

// NewDefaultKeyGenerator creates a DefaultKeyGenerator.
func NewDefaultKeyGenerator() *DefaultKeyGenerator {
	return &DefaultKeyGenerator{}
}

// Generate implements KeyGenerator.
func (g *DefaultKeyGenerator) Generate(prefix string, parts ...string) string {
	if len(parts) == 0 {
		return prefix
	}
	// Use strings.Builder to avoid the intermediate []string slice that
	// append([]string{prefix}, parts...) would allocate.
	var b strings.Builder
	b.WriteString(prefix)
	for _, p := range parts {
		b.WriteByte(':')
		b.WriteString(p)
	}
	return b.String()
}
