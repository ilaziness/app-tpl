package gen

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ilaziness/app-tpl/internal/database"
)

func getSQLiteColumns(ctx context.Context, db *database.DB, tableName string) ([]Column, error) {
	query := fmt.Sprintf("PRAGMA table_info(%q)", tableName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rawColumn struct {
		cid       int
		name      string
		dataType  string
		notNull   int
		dfltValue sql.NullString
		pk        int
	}

	var rawCols []rawColumn
	for rows.Next() {
		var r rawColumn
		if scanErr := rows.Scan(&r.cid, &r.name, &r.dataType, &r.notNull, &r.dfltValue, &r.pk); scanErr != nil {
			return nil, scanErr
		}
		rawCols = append(rawCols, r)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	pkCount := 0
	for _, r := range rawCols {
		if r.pk > 0 {
			pkCount++
		}
	}

	var columns []Column
	for _, r := range rawCols {
		primaryKey := r.pk > 0
		baseType := strings.ToUpper(strings.TrimSpace(r.dataType))
		if i := strings.IndexByte(baseType, '('); i >= 0 {
			baseType = strings.TrimSpace(baseType[:i])
		}
		// SQLite treats only INTEGER PRIMARY KEY as the rowid alias (autoincrement).
		autoIncrement := primaryKey && pkCount == 1 && baseType == "INTEGER"
		columns = append(columns, Column{
			Name:          r.name,
			Type:          r.dataType,
			Nullable:      r.notNull == 0 && !primaryKey,
			Default:       r.dfltValue.String,
			PrimaryKey:    primaryKey,
			AutoIncrement: autoIncrement,
		})
	}

	groups, err := getSQLiteUniqueGroups(ctx, db, tableName)
	if err != nil {
		return nil, err
	}
	applyUniqueConstraints(columns, groups)

	return columns, nil
}

func getSQLiteUniqueGroups(ctx context.Context, db *database.DB, tableName string) (map[string][]string, error) {
	query := fmt.Sprintf("PRAGMA index_list(%q)", tableName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type indexInfo struct {
		name   string
		unique int
		origin string
	}

	var indexes []indexInfo
	for rows.Next() {
		var seq, partial int
		var name, origin string
		var unique int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, err
		}
		if unique == 1 && origin != "pk" {
			indexes = append(indexes, indexInfo{name: name, unique: unique, origin: origin})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	for _, idx := range indexes {
		colQuery := fmt.Sprintf("SELECT seqno, name FROM pragma_index_info('%s') ORDER BY seqno", strings.ReplaceAll(idx.name, "'", "''"))
		colRows, err := db.QueryContext(ctx, colQuery)
		if err != nil {
			return nil, err
		}
		for colRows.Next() {
			var seqno int
			var colName string
			if err := colRows.Scan(&seqno, &colName); err != nil {
				colRows.Close()
				return nil, err
			}
			groups[idx.name] = append(groups[idx.name], colName)
		}
		if err := colRows.Err(); err != nil {
			colRows.Close()
			return nil, err
		}
		colRows.Close()
	}

	return groups, nil
}

func getSQLiteForeignKeyConstraints(ctx context.Context, db *database.DB, tableName string) ([]ForeignKeyConstraint, error) {
	query := fmt.Sprintf("PRAGMA foreign_key_list(%q)", tableName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type raw struct {
		id    int
		seq   int
		table string
		from  string
		to    string
	}
	var items []raw
	for rows.Next() {
		var id, seq int
		var table, from, to, onUpdate, onDelete sql.NullString
		var match sql.NullString
		if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}
		if !table.Valid {
			continue
		}
		items = append(items, raw{id: id, seq: seq, table: table.String, from: from.String, to: to.String})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	grouped := map[int]*ForeignKeyConstraint{}
	var order []int
	for _, item := range items {
		fk, ok := grouped[item.id]
		if !ok {
			fk = &ForeignKeyConstraint{
				Name:                fmt.Sprintf("fk_%d", item.id),
				ReferencedTableName: item.table,
			}
			grouped[item.id] = fk
			order = append(order, item.id)
		}
		fk.Columns = append(fk.Columns, item.from)
		fk.ReferencedColumns = append(fk.ReferencedColumns, item.to)
	}

	out := make([]ForeignKeyConstraint, 0, len(order))
	for _, id := range order {
		out = append(out, *grouped[id])
	}
	return out, nil
}

func listSQLiteTables(ctx context.Context, db *database.DB) ([]string, error) {
	query := "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
	return scanTableNames(ctx, db, query)
}
