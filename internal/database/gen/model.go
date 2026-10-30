// Package gen generates Go model code from database schema introspection.
package gen

import (
	"context"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ilaziness/app-tpl/internal/constant"
	"github.com/ilaziness/app-tpl/internal/database"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var tableNameRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func validateTableName(name string) error {
	if name == "" {
		return fmt.Errorf("table name is required")
	}
	if !tableNameRE.MatchString(name) {
		return fmt.Errorf("invalid table name %q", name)
	}
	return nil
}

// ValidateTableName reports whether a table name is safe for introspection.
func ValidateTableName(name string) error {
	return validateTableName(name)
}

var titleCaser = cases.Title(language.Und)

// Column describes one database column.
type Column struct {
	Name          string
	Type          string
	ColumnType    string // MySQL COLUMN_TYPE (e.g. tinyint(1), enum('a','b'))
	UDTName       string // PostgreSQL udt_name
	Nullable      bool
	Default       string
	Comment       string
	PrimaryKey    bool
	AutoIncrement bool
	Unique        bool
	UniqueGroup   string
	Unsigned      bool
	Length        int
	ArrayDims     int
	EnumValues    []string
}

// ForeignKeyConstraint describes a (possibly composite) foreign key.
type ForeignKeyConstraint struct {
	Name                string
	Columns             []string
	ReferencedTableName string
	ReferencedColumns   []string
}

// GenerateOptions configures model code generation.
type GenerateOptions struct {
	OutputDir     string
	PackageName   string
	Driver        string
	Schema        string
	JSONTags      bool
	ValidatorTags bool
	WithRelations bool
	NoExpand      bool
	DryRun        bool
	Exclude       map[string]struct{}
	Aliases       map[string]string
	Relations     []Relation
	Out           io.Writer // dry-run destination; defaults to os.Stdout
}

// GenerateModels generates model files for the given tables.
func GenerateModels(ctx context.Context, db *database.DB, driver string, tables []string, opts GenerateOptions) error {
	if len(tables) == 0 {
		return nil
	}
	opts.Driver = driver

	schema, err := resolveSchema(ctx, db, driver, opts.Schema)
	if err != nil {
		return err
	}
	opts.Schema = schema

	if err := validateRelations(opts.Relations); err != nil {
		return err
	}

	relations := opts.Relations
	if !opts.NoExpand {
		tables = expandTablesForRelations(tables, relations)
	}

	if opts.WithRelations {
		physical, err := getPhysicalRelations(ctx, db, driver, schema, tables)
		if err != nil {
			return fmt.Errorf("get physical relations: %w", err)
		}
		relations, err = mergeRelations(physical, relations)
		if err != nil {
			return err
		}
		if !opts.NoExpand {
			tables = expandTablesForRelations(tables, relations)
		}
	}

	columnsByTable := make(map[string][]Column, len(tables))
	loadTable := func(tableName string) ([]Column, error) {
		if cols, ok := columnsByTable[tableName]; ok {
			return cols, nil
		}
		cols, err := GetTableColumns(ctx, db, driver, schema, tableName)
		if err != nil {
			return nil, err
		}
		columnsByTable[tableName] = cols
		return cols, nil
	}
	for _, tableName := range tables {
		if _, err := loadTable(tableName); err != nil {
			return fmt.Errorf("read table %s: %w", tableName, err)
		}
	}
	for _, rel := range relations {
		for _, t := range []string{rel.SourceTable, rel.TargetTable, rel.Through} {
			if t == "" {
				continue
			}
			if _, err := loadTable(t); err != nil {
				return fmt.Errorf("read relation table %s: %w", t, err)
			}
		}
	}

	uniqueSingle, pkCols := collectUniqueness(columnsByTable)
	annotateHasOne(relations, uniqueSingle, pkCols)

	autoM2M, suppress := detectPureM2MTables(columnsByTable, relations)
	if len(autoM2M) > 0 {
		relations = append(relations, autoM2M...)
	}
	relations = applySkipReverse(relations, suppress)
	relations, err = mergeRelations(nil, relations)
	if err != nil {
		return err
	}

	relations, err = resolveM2MJoins(relations)
	if err != nil {
		return err
	}

	if err := validateRelationColumns(columnsByTable, relations); err != nil {
		return err
	}

	opts.Aliases = UniqueAliases(tables)

	if !opts.DryRun {
		if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	out := opts.Out
	if out == nil {
		out = os.Stdout
	}

	for _, tableName := range tables {
		if _, skip := opts.Exclude[tableName]; skip {
			continue
		}
		columns, err := loadTable(tableName)
		if err != nil {
			return fmt.Errorf("generate model for table %s: %w", tableName, err)
		}
		if err := writeModel(opts, tableName, columns, relations, out); err != nil {
			return fmt.Errorf("generate model for table %s: %w", tableName, err)
		}
	}

	return nil
}

func resolveSchema(ctx context.Context, db *database.DB, driver, schema string) (string, error) {
	switch driver {
	case constant.DriverPostgres, constant.DriverPostgreSQL:
		return resolvePostgresSchema(ctx, db, schema)
	default:
		return schema, nil
	}
}

func writeModel(opts GenerateOptions, tableName string, columns []Column, relations []Relation, out io.Writer) error {
	modelName := ToCamelCase(tableName)
	code := GenerateModelCode(opts, modelName, tableName, columns, relations)

	formatted, err := format.Source([]byte(code))
	if err != nil {
		return fmt.Errorf("format generated code: %w", err)
	}

	filename := strings.ToLower(tableName) + ".gen.go"
	if opts.DryRun {
		_, err := fmt.Fprintf(out, "// file: %s\n%s", filename, formatted)
		return err
	}

	return os.WriteFile(filepath.Join(opts.OutputDir, filename), formatted, 0o644)
}

// GetTables returns user table names for the configured driver.
func GetTables(ctx context.Context, db *database.DB, driver string) ([]string, error) {
	return GetTablesInSchema(ctx, db, driver, "")
}

// GetTablesInSchema returns user table names, optionally scoped to a PostgreSQL schema.
func GetTablesInSchema(ctx context.Context, db *database.DB, driver, schema string) ([]string, error) {
	switch driver {
	case constant.DriverMySQL:
		return listMySQLTables(ctx, db)
	case constant.DriverPostgres, constant.DriverPostgreSQL:
		resolved, err := resolvePostgresSchema(ctx, db, schema)
		if err != nil {
			return nil, err
		}
		return listPostgresTables(ctx, db, resolved)
	case constant.DriverSQLite, constant.DriverSQLite3:
		return listSQLiteTables(ctx, db)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}
}

// GetTableColumns returns column metadata for a table.
func GetTableColumns(ctx context.Context, db *database.DB, driver, schema, tableName string) ([]Column, error) {
	if err := validateTableName(tableName); err != nil {
		return nil, err
	}

	switch driver {
	case constant.DriverMySQL:
		return getMySQLColumns(ctx, db, tableName)
	case constant.DriverPostgres, constant.DriverPostgreSQL:
		resolved, err := resolvePostgresSchema(ctx, db, schema)
		if err != nil {
			return nil, err
		}
		return getPostgresColumns(ctx, db, resolved, tableName)
	case constant.DriverSQLite, constant.DriverSQLite3:
		return getSQLiteColumns(ctx, db, tableName)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}
}

func scanUniqueGroups(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) (map[string][]string, error) {
	groups := make(map[string][]string)
	for rows.Next() {
		var constraintName, columnName string
		if err := rows.Scan(&constraintName, &columnName); err != nil {
			return nil, err
		}
		groups[constraintName] = append(groups[constraintName], columnName)
	}
	return groups, rows.Err()
}

func applyUniqueConstraints(columns []Column, groups map[string][]string) {
	if len(groups) == 0 {
		return
	}

	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		li, lj := len(groups[keys[i]]), len(groups[keys[j]])
		if li != lj {
			return li < lj
		}
		return keys[i] < keys[j]
	})

	for _, group := range keys {
		cols := groups[group]
		for _, colName := range cols {
			for i := range columns {
				if columns[i].Name != colName || columns[i].Unique {
					continue
				}
				if columns[i].PrimaryKey && len(cols) == 1 {
					continue
				}
				columns[i].Unique = true
				if len(cols) > 1 {
					columns[i].UniqueGroup = group
				}
			}
		}
	}
}

func collectUniqueness(columnsByTable map[string][]Column) (uniqueSingle map[string]map[string]bool, pkCols map[string]map[string]bool) {
	uniqueSingle = make(map[string]map[string]bool)
	pkCols = make(map[string]map[string]bool)
	for table, columns := range columnsByTable {
		u := make(map[string]bool)
		p := make(map[string]bool)
		for _, col := range columns {
			if col.PrimaryKey {
				p[col.Name] = true
			}
			if col.Unique && col.UniqueGroup == "" {
				u[col.Name] = true
			}
		}
		uniqueSingle[table] = u
		pkCols[table] = p
	}
	return uniqueSingle, pkCols
}

func getForeignKeyConstraints(ctx context.Context, db *database.DB, driver, schema, tableName string) ([]ForeignKeyConstraint, error) {
	switch driver {
	case constant.DriverMySQL:
		return getMySQLForeignKeyConstraints(ctx, db, tableName)
	case constant.DriverPostgres, constant.DriverPostgreSQL:
		return getPostgresForeignKeyConstraints(ctx, db, schema, tableName)
	case constant.DriverSQLite, constant.DriverSQLite3:
		return getSQLiteForeignKeyConstraints(ctx, db, tableName)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}
}

func getPhysicalRelations(ctx context.Context, db *database.DB, driver, schema string, tables []string) ([]Relation, error) {
	var relations []Relation
	for _, tableName := range tables {
		fks, err := getForeignKeyConstraints(ctx, db, driver, schema, tableName)
		if err != nil {
			return nil, fmt.Errorf("get foreign keys for table %s: %w", tableName, err)
		}
		for _, fk := range fks {
			relations = append(relations, Relation{
				SourceTable:  tableName,
				SourceColumn: strings.Join(fk.Columns, "+"),
				TargetTable:  fk.ReferencedTableName,
				TargetColumn: strings.Join(fk.ReferencedColumns, "+"),
			})
		}
	}
	return relations, nil
}

func validateRelationColumns(columnsByTable map[string][]Column, relations []Relation) error {
	type tableMeta struct {
		columns map[string]struct{}
		fields  map[string]struct{}
	}
	meta := make(map[string]tableMeta, len(columnsByTable))
	load := func(table string) (tableMeta, error) {
		if m, ok := meta[table]; ok {
			return m, nil
		}
		cols, ok := columnsByTable[table]
		if !ok {
			return tableMeta{}, fmt.Errorf("unknown table %s", table)
		}
		m := tableMeta{
			columns: make(map[string]struct{}, len(cols)),
			fields:  map[string]struct{}{"BaseModel": {}},
		}
		for _, column := range cols {
			m.columns[column.Name] = struct{}{}
			m.fields[ToCamelCase(column.Name)] = struct{}{}
		}
		meta[table] = m
		return m, nil
	}

	for _, relation := range relations {
		field, reverseField := relationFieldNames(relation)

		if relation.Type == RelationManyToMany {
			src, err := load(relation.SourceTable)
			if err != nil {
				return err
			}
			if _, ok := src.fields[field]; ok {
				return fmt.Errorf("relation field conflict with column: %s.%s", relation.SourceTable, field)
			}
			tgt, err := load(relation.TargetTable)
			if err != nil {
				return err
			}
			if _, ok := tgt.fields[reverseField]; ok {
				return fmt.Errorf("relation field conflict with column: %s.%s", relation.TargetTable, reverseField)
			}
			continue
		}

		src, err := load(relation.SourceTable)
		if err != nil {
			return err
		}
		for _, col := range splitCompositeColumns(relation.SourceColumn) {
			if _, ok := src.columns[col]; !ok {
				return fmt.Errorf("relation source column not found: %s.%s", relation.SourceTable, col)
			}
		}
		if _, ok := src.fields[field]; ok {
			return fmt.Errorf("relation field conflict with column: %s.%s", relation.SourceTable, field)
		}

		tgt, err := load(relation.TargetTable)
		if err != nil {
			return err
		}
		for _, col := range splitCompositeColumns(relation.TargetColumn) {
			if _, ok := tgt.columns[col]; !ok {
				return fmt.Errorf("relation target column not found: %s.%s", relation.TargetTable, col)
			}
		}
		if !relation.SkipReverse {
			if _, ok := tgt.fields[reverseField]; ok {
				return fmt.Errorf("relation field conflict with column: %s.%s", relation.TargetTable, reverseField)
			}
		}
	}
	return nil
}

// expandTablesForRelations ensures that both ends of every relation are
// generated so that referenced model types are always available.
func expandTablesForRelations(tables []string, relations []Relation) []string {
	seen := make(map[string]struct{}, len(tables))
	for _, tableName := range tables {
		seen[tableName] = struct{}{}
	}
	for _, relation := range relations {
		for _, name := range []string{relation.SourceTable, relation.TargetTable, relation.Through} {
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			tables = append(tables, name)
			seen[name] = struct{}{}
		}
	}
	return tables
}
