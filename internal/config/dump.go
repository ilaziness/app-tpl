package config

import (
	"fmt"

	"github.com/go-viper/mapstructure/v2"
	"gopkg.in/yaml.v3"
)

// MarshalYAMLMasked returns the effective config as YAML with secrets masked.
func (c *Config) MarshalYAMLMasked() ([]byte, error) {
	var m map[string]any
	if err := mapstructure.Decode(c.MaskedCopy(), &m); err != nil {
		return nil, fmt.Errorf("failed to decode config for dump: %w", err)
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config yaml: %w", err)
	}
	return out, nil
}
