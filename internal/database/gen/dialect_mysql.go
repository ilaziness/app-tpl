package gen

import (
	"context"
	"database/sql"
	"strings"

	"github.com/ilaziness/app-tpl/internal/database"
)

func getMySQLColumns(ctx context.Context, db *database.DB, tableName string) ([]Column, error) {
	query := `
		SELECT c.column_name, c.data_type, c.column_type, c.is_nullable, c.column_default, c.column_comment, c.extra,
			COALESCE(p.is_primary, 0) AS is_primary_key
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT kcu.column_name, 1 AS is_primary
			FROM information_schema.key_column_usage kcu
			JOIN information_schema.table_constraints tc
				ON kcu.table_schema = tc.table_schema
				AND kcu.table_name = tc.table_name
				AND kcu.constraint_name = tc.constraint_name
			WHERE kcu.table_schema = DATABASE() AND kcu.table_name = ? AND tc.constraint_type = 'PRIMARY KEY'
		) p ON c.column_name = p.column_name
		WHERE c.table_schema = DATABASE() AND c.table_name = ?
		ORDER BY c.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, tableName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := scanMySQLColumns(rows)
	if err != nil {
		return nil, err
	}

	groups, err := getMySQLUniqueGroups(ctx, db, tableName)
	if err != nil {
		return nil, err
	}
	applyUniqueConstraints(columns, groups)

	return columns, nil
}

func scanMySQLColumns(rows *sql.Rows) ([]Column, error) {
	var columns []Column
	for rows.Next() {
		var columnName, dataType, columnType, isNullable sql.NullString
		var columnDefault, columnComment, extra sql.NullString
		var isPrimaryKey int

		if err := rows.Scan(&columnName, &dataType, &columnType, &isNullable, &columnDefault, &columnComment, &extra, &isPrimaryKey); err != nil {
			return nil, err
		}

		columns = append(columns, Column{
			Name:          columnName.String,
			Type:          dataType.String,
			ColumnType:    columnType.String,
			Nullable:      isNullable.String == "YES",
			Default:       columnDefault.String,
			Comment:       columnComment.String,
			PrimaryKey:    isPrimaryKey == 1,
			AutoIncrement: strings.Contains(strings.ToLower(extra.String), "auto_increment"),
			Unsigned:      strings.Contains(strings.ToLower(columnType.String), "unsigned"),
		})
	}

	return columns, rows.Err()
}

func getMySQLUniqueGroups(ctx context.Context, db *database.DB, tableName string) (map[string][]string, error) {
	query := `
		SELECT tc.constraint_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.table_schema = kcu.table_schema
			AND tc.table_name = kcu.table_name
			AND tc.constraint_name = kcu.constraint_name
		WHERE tc.table_schema = DATABASE() AND tc.table_name = ? AND tc.constraint_type = 'UNIQUE'
		ORDER BY tc.constraint_name, kcu.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanUniqueGroups(rows)
}

func getMySQLForeignKeyConstraints(ctx context.Context, db *database.DB, tableName string) ([]ForeignKeyConstraint, error) {
	query := `
		SELECT kcu.constraint_name, kcu.column_name, kcu.referenced_table_name, kcu.referenced_column_name, kcu.ordinal_position
		FROM information_schema.key_column_usage kcu
		WHERE kcu.table_schema = DATABASE() AND kcu.table_name = ?
			AND kcu.referenced_table_name IS NOT NULL
		ORDER BY kcu.constraint_name, kcu.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type row struct {
		constraint string
		column     string
		refTable   string
		refColumn  string
		ordinal    int
	}
	var raw []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.constraint, &r.column, &r.refTable, &r.refColumn, &r.ordinal); err != nil {
			return nil, err
		}
		raw = append(raw, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	grouped := map[string]*ForeignKeyConstraint{}
	var order []string
	for _, r := range raw {
		fk, ok := grouped[r.constraint]
		if !ok {
			fk = &ForeignKeyConstraint{
				Name:                r.constraint,
				ReferencedTableName: r.refTable,
				Columns:             nil,
				ReferencedColumns:   nil,
			}
			grouped[r.constraint] = fk
			order = append(order, r.constraint)
		}
		fk.Columns = append(fk.Columns, r.column)
		fk.ReferencedColumns = append(fk.ReferencedColumns, r.refColumn)
	}

	out := make([]ForeignKeyConstraint, 0, len(order))
	for _, name := range order {
		out = append(out, *grouped[name])
	}
	return out, nil
}

func listMySQLTables(ctx context.Context, db *database.DB) ([]string, error) {
	query := "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'"
	return scanTableNames(ctx, db, query)
}

func scanTableNames(ctx context.Context, db *database.DB, query string, args ...any) ([]string, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		if tableName == "bun_migrations" || tableName == "bun_migration_locks" {
			continue
		}
		tables = append(tables, tableName)
	}
	return tables, rows.Err()
}
