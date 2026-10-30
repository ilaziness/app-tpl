package gen

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ilaziness/app-tpl/internal/database"
)

func resolvePostgresSchema(ctx context.Context, db *database.DB, schema string) (string, error) {
	if schema != "" {
		if err := validateTableName(schema); err != nil {
			return "", fmt.Errorf("invalid schema: %w", err)
		}
		return schema, nil
	}
	var current string
	if err := db.QueryRowContext(ctx, "SELECT current_schema()").Scan(&current); err != nil {
		return "", fmt.Errorf("current_schema: %w", err)
	}
	if current == "" {
		return "public", nil
	}
	return current, nil
}

func getPostgresColumns(ctx context.Context, db *database.DB, schema, tableName string) ([]Column, error) {
	query := `
		SELECT c.column_name, c.data_type, c.udt_name, c.is_nullable, c.column_default,
			COALESCE(pgd.description, '') AS column_comment,
			c.is_identity,
			COALESCE(p.is_primary, 0) AS is_primary_key,
			COALESCE(c.character_maximum_length, 0) AS char_len,
			COALESCE(
				(SELECT a.attndims
				 FROM pg_catalog.pg_attribute a
				 JOIN pg_catalog.pg_class cls ON cls.oid = a.attrelid
				 JOIN pg_catalog.pg_namespace nsp ON nsp.oid = cls.relnamespace
				 WHERE nsp.nspname = $1 AND cls.relname = $2 AND a.attname = c.column_name AND a.attnum > 0 AND NOT a.attisdropped),
				0
			) AS array_dims
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT kcu.column_name, 1 AS is_primary
			FROM information_schema.key_column_usage kcu
			JOIN information_schema.table_constraints tc
				ON kcu.table_schema = tc.table_schema
				AND kcu.table_name = tc.table_name
				AND kcu.constraint_name = tc.constraint_name
			WHERE kcu.table_schema = $1 AND kcu.table_name = $2 AND tc.constraint_type = 'PRIMARY KEY'
		) p ON c.column_name = p.column_name
		LEFT JOIN pg_catalog.pg_statio_all_tables st
			ON st.schemaname = c.table_schema AND st.relname = c.table_name
		LEFT JOIN pg_catalog.pg_description pgd
			ON pgd.objoid = st.relid AND pgd.objsubid = c.ordinal_position
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := scanPostgresColumns(rows)
	if err != nil {
		return nil, err
	}

	if err := attachPostgresEnums(ctx, db, schema, columns); err != nil {
		return nil, err
	}

	groups, err := getPostgresUniqueGroups(ctx, db, schema, tableName)
	if err != nil {
		return nil, err
	}
	applyUniqueConstraints(columns, groups)

	return columns, nil
}

func scanPostgresColumns(rows *sql.Rows) ([]Column, error) {
	var columns []Column
	for rows.Next() {
		var columnName, dataType, udtName, isNullable, columnDefault, columnComment, isIdentity sql.NullString
		var isPrimaryKey, charLen, arrayDims int

		if err := rows.Scan(&columnName, &dataType, &udtName, &isNullable, &columnDefault, &columnComment, &isIdentity, &isPrimaryKey, &charLen, &arrayDims); err != nil {
			return nil, err
		}

		autoIncrement := isIdentity.Valid && isIdentity.String == "YES"
		if !autoIncrement {
			autoIncrement = strings.Contains(columnDefault.String, "nextval")
		}

		columns = append(columns, Column{
			Name:          columnName.String,
			Type:          dataType.String,
			UDTName:       udtName.String,
			Nullable:      isNullable.String == "YES",
			Default:       columnDefault.String,
			Comment:       columnComment.String,
			PrimaryKey:    isPrimaryKey == 1,
			AutoIncrement: autoIncrement,
			Length:        charLen,
			ArrayDims:     arrayDims,
		})
	}

	return columns, rows.Err()
}

func attachPostgresEnums(ctx context.Context, db *database.DB, schema string, columns []Column) error {
	enumTypes := map[string]struct{}{}
	for _, col := range columns {
		if strings.EqualFold(col.Type, "USER-DEFINED") && col.UDTName != "" {
			enumTypes[col.UDTName] = struct{}{}
		}
	}
	if len(enumTypes) == 0 {
		return nil
	}

	cache := map[string][]string{}
	for udt := range enumTypes {
		if err := validateTableName(udt); err != nil {
			continue
		}
		query := `
			SELECT e.enumlabel
			FROM pg_catalog.pg_type t
			JOIN pg_catalog.pg_enum e ON t.oid = e.enumtypid
			JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
			WHERE t.typname = $1 AND (n.nspname = $2 OR n.nspname = 'public')
			ORDER BY e.enumsortorder
		`
		rows, err := db.QueryContext(ctx, query, udt, schema)
		if err != nil {
			return err
		}
		var labels []string
		for rows.Next() {
			var label string
			if err := rows.Scan(&label); err != nil {
				rows.Close()
				return err
			}
			labels = append(labels, label)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if len(labels) > 0 {
			cache[udt] = labels
		}
	}

	for i := range columns {
		if vals, ok := cache[columns[i].UDTName]; ok {
			columns[i].EnumValues = append([]string(nil), vals...)
			if columns[i].Type == "" || strings.EqualFold(columns[i].Type, "USER-DEFINED") {
				columns[i].Type = columns[i].UDTName
			}
		}
	}
	return nil
}

func getPostgresUniqueGroups(ctx context.Context, db *database.DB, schema, tableName string) (map[string][]string, error) {
	query := `
		SELECT tc.constraint_name, kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.table_schema = kcu.table_schema
			AND tc.table_name = kcu.table_name
			AND tc.constraint_name = kcu.constraint_name
		WHERE tc.table_schema = $1 AND tc.table_name = $2 AND tc.constraint_type = 'UNIQUE'
		ORDER BY tc.constraint_name, kcu.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanUniqueGroups(rows)
}

func getPostgresForeignKeyConstraints(ctx context.Context, db *database.DB, schema, tableName string) ([]ForeignKeyConstraint, error) {
	// Pair local/referenced columns by ordinal_position via referential_constraints.
	query := `
		SELECT tc.constraint_name, kcu.column_name, rel_kcu.table_name, rel_kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_schema = kcu.constraint_schema
			AND tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
			AND tc.table_name = kcu.table_name
		JOIN information_schema.referential_constraints rc
			ON tc.constraint_schema = rc.constraint_schema
			AND tc.constraint_name = rc.constraint_name
		JOIN information_schema.key_column_usage rel_kcu
			ON rc.unique_constraint_schema = rel_kcu.constraint_schema
			AND rc.unique_constraint_name = rel_kcu.constraint_name
			AND kcu.ordinal_position = rel_kcu.ordinal_position
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = $1 AND tc.table_name = $2
		ORDER BY tc.constraint_name, kcu.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, schema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grouped := map[string]*ForeignKeyConstraint{}
	var order []string
	for rows.Next() {
		var constraint, column, refTable, refColumn string
		if err := rows.Scan(&constraint, &column, &refTable, &refColumn); err != nil {
			return nil, err
		}
		fk, ok := grouped[constraint]
		if !ok {
			fk = &ForeignKeyConstraint{
				Name:                constraint,
				ReferencedTableName: refTable,
			}
			grouped[constraint] = fk
			order = append(order, constraint)
		}
		fk.Columns = append(fk.Columns, column)
		fk.ReferencedColumns = append(fk.ReferencedColumns, refColumn)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]ForeignKeyConstraint, 0, len(order))
	for _, name := range order {
		out = append(out, *grouped[name])
	}
	return out, nil
}

func listPostgresTables(ctx context.Context, db *database.DB, schema string) ([]string, error) {
	query := "SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE'"
	return scanTableNames(ctx, db, query, schema)
}
