package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/ilaziness/app-tpl/internal/config"
	"github.com/spf13/cobra"
)

var configShowCheck bool

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the configuration file",
	Long: `Load configuration with the same rules as serve, run Validate(), and print
a short summary of environment, source files, and enabled modules.`,
	RunE: runConfigValidate,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the effective configuration",
	Long: `Print the effective (merged) configuration as YAML with secrets masked.
By default does not run Validate (useful for inspecting incomplete configs).
Use --check to validate before printing.`,
	RunE: runConfigShow,
}

func init() {
	configShowCmd.Flags().BoolVar(&configShowCheck, "check", false, "validate configuration before printing")

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management commands",
	}
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	cfg, meta, err := config.LoadWithMeta(cfgFile, env)
	if err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Configuration is valid")
	printLoadSummary(out, cfg, meta)
	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, meta, err := config.LoadWithMeta(cfgFile, env)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if configShowCheck {
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("configuration validation failed: %w", err)
		}
	}

	out := cmd.OutOrStdout()
	printLoadSummary(out, cfg, meta)
	fmt.Fprintln(out, "---")
	yamlBytes, err := cfg.MarshalYAMLMasked()
	if err != nil {
		return fmt.Errorf("failed to dump config: %w", err)
	}
	fmt.Fprint(out, string(yamlBytes))
	return nil
}

func printLoadSummary(out io.Writer, cfg *config.Config, meta config.LoadMeta) {
	fmt.Fprintf(out, "App: %s %s\n", cfg.App.Name, cfg.App.Version)
	fmt.Fprintf(out, "Environment: %s\n", cfg.App.Env)
	if meta.BasePath != "" {
		fmt.Fprintf(out, "Base config: %s\n", meta.BasePath)
	} else {
		fmt.Fprintln(out, "Base config: (defaults only)")
	}
	if meta.OverlayPath != "" {
		fmt.Fprintf(out, "Overlay (%s): %s\n", meta.OverlayEnv, meta.OverlayPath)
	} else if meta.OverlayEnv != "" {
		fmt.Fprintf(out, "Overlay (%s): (file not found)\n", meta.OverlayEnv)
	}
	if len(meta.DotenvPaths) > 0 {
		fmt.Fprintf(out, "Dotenv: %s\n", strings.Join(meta.DotenvPaths, ", "))
	}
	if mods := cfg.EnabledModules(); len(mods) > 0 {
		fmt.Fprintf(out, "Enabled modules: %s\n", strings.Join(mods, ", "))
	}
}
