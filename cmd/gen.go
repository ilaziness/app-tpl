package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/ilaziness/app-tpl/internal/config"
	"github.com/ilaziness/app-tpl/internal/database"
	"github.com/ilaziness/app-tpl/internal/database/gen"
	"github.com/spf13/cobra"
)

var (
	genTables        []string
	genExclude       []string
	genOutput        string
	genPackage       string
	genSchema        string
	genJSONTags      bool
	genValidatorTags bool
	genWithRelations bool
	genNoExpand      bool
	genDryRun        bool
	genRelationsFile string
)

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Code generation commands",
	Long:  `Code generation commands.`,
}

var genModelCmd = &cobra.Command{
	Use:   "model",
	Short: "Generate model from database",
	Long:  `Generate Go model files from database tables using Bun schema.`,
	RunE:  runGenModel,
}

func init() {
	genModelCmd.Flags().StringSliceVar(&genTables, "table", nil, "Database table name(s); repeatable or comma-separated (omit to generate all tables)")
	genModelCmd.Flags().StringSliceVar(&genExclude, "exclude", nil, "Table name(s) to skip writing; repeatable or comma-separated")
	genModelCmd.Flags().StringVar(&genOutput, "output", "./internal/model", "Generated model output directory (string path)")
	genModelCmd.Flags().StringVar(&genPackage, "package", "model", "Generated model package name (string)")
	genModelCmd.Flags().StringVar(&genSchema, "schema", "", "PostgreSQL schema (default: current_schema())")
	genModelCmd.Flags().BoolVar(&genJSONTags, "json-tags", true, "Whether to add JSON tags (bool: true or false)")
	genModelCmd.Flags().BoolVar(&genValidatorTags, "validator-tags", false, "Whether to add validator tags (bool: true or false)")
	genModelCmd.Flags().BoolVar(&genWithRelations, "with-relations", false, "Whether to generate relationships from foreign keys (bool: true or false)")
	genModelCmd.Flags().BoolVar(&genNoExpand, "no-expand", false, "Do not generate related tables referenced by relations")
	genModelCmd.Flags().BoolVar(&genDryRun, "dry-run", false, "Print generated code to stdout without writing files")
	genModelCmd.Flags().StringVar(&genRelationsFile, "relations", "./configs/model-relations.yaml", "Business relation definition YAML file (string path)")

	genCmd.AddCommand(genModelCmd)
	rootCmd.AddCommand(genCmd)
}

func splitFlagValues(values []string) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, ok := seen[part]; ok {
				continue
			}
			seen[part] = struct{}{}
			out = append(out, part)
		}
	}
	return out
}

func runGenModel(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadWithEnv(cfgFile, env)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logInst, err := newLoggerFromConfig(cfg)
	if err != nil {
		return err
	}

	db, err := database.NewDB(cfg, logInst.Logger)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()

	tableFlags := splitFlagValues(genTables)
	var tables []string
	if len(tableFlags) > 0 {
		for _, name := range tableFlags {
			if validateErr := gen.ValidateTableName(name); validateErr != nil {
				return fmt.Errorf("invalid --table %q: %w", name, validateErr)
			}
		}
		tables = tableFlags
	} else {
		tables, err = gen.GetTablesInSchema(ctx, db, cfg.Database.Driver, genSchema)
		if err != nil {
			return fmt.Errorf("failed to get tables: %w", err)
		}
	}

	if len(tables) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No tables found")
		return nil
	}

	relationsRequired := cmd.Flags().Changed("relations")
	relations, err := gen.LoadRelationsOptional(genRelationsFile, relationsRequired)
	if err != nil {
		return fmt.Errorf("load --relations: %w", err)
	}

	exclude := map[string]struct{}{}
	for _, name := range splitFlagValues(genExclude) {
		if validateErr := gen.ValidateTableName(name); validateErr != nil {
			return fmt.Errorf("invalid --exclude %q: %w", name, validateErr)
		}
		exclude[name] = struct{}{}
	}

	opts := gen.GenerateOptions{
		OutputDir:     genOutput,
		PackageName:   genPackage,
		Schema:        genSchema,
		JSONTags:      genJSONTags,
		ValidatorTags: genValidatorTags,
		WithRelations: genWithRelations,
		NoExpand:      genNoExpand,
		DryRun:        genDryRun,
		Exclude:       exclude,
		Relations:     relations,
		Out:           cmd.OutOrStdout(),
	}

	if err := gen.GenerateModels(ctx, db, cfg.Database.Driver, tables, opts); err != nil {
		return err
	}

	if genDryRun {
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nSuccessfully generated model(s) into %s\n", genOutput)
	return nil
}
