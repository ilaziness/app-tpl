package gen

import (
	"bytes"
	"context"
	"go/format"
	"os"
	"path/filepath"
	"testing"

	"github.com/ilaziness/app-tpl/internal/constant"
	"github.com/ilaziness/app-tpl/internal/database/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTables_returnsUserTablesFromSQLite(t *testing.T) {
	db := testutil.OpenBunDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `CREATE TABLE sample_items (id INTEGER PRIMARY KEY, title TEXT NOT NULL)`)
	require.NoError(t, err)

	tables, err := GetTables(ctx, db, constant.DriverSQLite)
	require.NoError(t, err)
	assert.Contains(t, tables, "sample_items")
}

func TestGetTableColumns_readsSQLiteColumns(t *testing.T) {
	db := testutil.OpenBunDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `CREATE TABLE sample_items (
		id INTEGER PRIMARY KEY,
		title TEXT NOT NULL,
		quantity INTEGER
	)`)
	require.NoError(t, err)

	columns, err := GetTableColumns(ctx, db, constant.DriverSQLite, "", "sample_items")
	require.NoError(t, err)
	require.Len(t, columns, 3)

	assert.Equal(t, "id", columns[0].Name)
	assert.Equal(t, "INTEGER", columns[0].Type)
	assert.True(t, columns[0].PrimaryKey)
	assert.True(t, columns[0].AutoIncrement)

	assert.Equal(t, "title", columns[1].Name)
	assert.False(t, columns[1].Nullable)

	assert.Equal(t, "quantity", columns[2].Name)
	assert.True(t, columns[2].Nullable)
}

func TestGetTableColumns_sqliteAutoincrementOnlyForIntegerPK(t *testing.T) {
	db := testutil.OpenBunDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `CREATE TABLE int_pk (id INT PRIMARY KEY, title TEXT)`)
	require.NoError(t, err)

	columns, err := GetTableColumns(ctx, db, constant.DriverSQLite, "", "int_pk")
	require.NoError(t, err)
	id := findColumn(columns, "id")
	require.NotNil(t, id)
	assert.True(t, id.PrimaryKey)
	assert.False(t, id.AutoIncrement)
}

func TestGetTableColumns_readsSQLiteUniqueConstraints(t *testing.T) {
	db := testutil.OpenBunDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `CREATE TABLE sample_items (
		id INTEGER PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		first_name TEXT NOT NULL,
		last_name TEXT NOT NULL,
		UNIQUE(first_name, last_name)
	)`)
	require.NoError(t, err)

	columns, err := GetTableColumns(ctx, db, constant.DriverSQLite, "", "sample_items")
	require.NoError(t, err)

	email := findColumn(columns, "email")
	require.NotNil(t, email)
	assert.True(t, email.Unique)
	assert.Empty(t, email.UniqueGroup)

	firstName := findColumn(columns, "first_name")
	lastName := findColumn(columns, "last_name")
	require.NotNil(t, firstName)
	require.NotNil(t, lastName)
	assert.True(t, firstName.Unique)
	assert.True(t, lastName.Unique)
	assert.NotEmpty(t, firstName.UniqueGroup)
	assert.Equal(t, firstName.UniqueGroup, lastName.UniqueGroup)
}

func findColumn(columns []Column, name string) *Column {
	for i := range columns {
		if columns[i].Name == name {
			return &columns[i]
		}
	}
	return nil
}

func TestGenerateModelCode_addsUniqueTag(t *testing.T) {
	code := GenerateModelCode(GenerateOptions{
		PackageName: "model",
		Driver:      constant.DriverMySQL,
		JSONTags:    true,
	}, "User", "users", []Column{
		{Name: "id", Type: "bigint", Nullable: false, PrimaryKey: true, AutoIncrement: true},
		{Name: "email", Type: "varchar", Nullable: false, Unique: true},
		{Name: "first_name", Type: "varchar", Nullable: true, Unique: true, UniqueGroup: "uk_name"},
	}, nil)

	assert.Contains(t, code, "\tEmail string `bun:\"email,notnull,unique\" json:\"email\"`")
	assert.Contains(t, code, "\tFirstName *string `bun:\"first_name,unique:uk_name\" json:\"first_name\"`")
}

func TestGenerateModelCode_includesBunAndJSONTags(t *testing.T) {
	code := GenerateModelCode(GenerateOptions{
		PackageName:   "model",
		Driver:        constant.DriverSQLite,
		JSONTags:      true,
		ValidatorTags: false,
	}, "SampleItem", "sample_items", []Column{
		{Name: "id", Type: "INTEGER", Nullable: false, PrimaryKey: true, AutoIncrement: true},
		{Name: "title", Type: "TEXT", Nullable: false},
	}, nil)

	assert.Contains(t, code, "type SampleItem struct")
	assert.Contains(t, code, "\tID int64 `bun:\"id,pk,autoincrement\" json:\"id\"`")
	assert.Contains(t, code, "\tTitle string `bun:\"title,notnull\" json:\"title\"`")
	assert.Contains(t, code, `bun:"table:sample_items,alias:si"`)
	assert.Contains(t, code, "// Database driver: sqlite")
}

func TestGenerateModelCode_hidesPasswordJSON(t *testing.T) {
	code := GenerateModelCode(GenerateOptions{
		PackageName: "model",
		Driver:      constant.DriverMySQL,
		JSONTags:    true,
	}, "Users", "users", []Column{
		{Name: "id", Type: "bigint", Nullable: false},
		{Name: "password", Type: "varchar", Nullable: false},
	}, nil)

	assert.Contains(t, code, "\tPassword string `bun:\"password,notnull\" json:\"-\"`")
	assert.NotContains(t, code, `"time"`)
	assert.NotContains(t, code, `json:"password"`)
}

func TestIsSensitiveColumn(t *testing.T) {
	assert.True(t, isSensitiveColumn("password"))
	assert.True(t, isSensitiveColumn("api_key"))
	assert.False(t, isSensitiveColumn("username"))
}

func TestGenerateModels_writesGofmtFormattedFiles(t *testing.T) {
	db := testutil.OpenBunDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `CREATE TABLE sample_items (
		id INTEGER PRIMARY KEY,
		title TEXT NOT NULL
	)`)
	require.NoError(t, err)

	outputDir := t.TempDir()
	err = GenerateModels(ctx, db, constant.DriverSQLite, []string{"sample_items"}, GenerateOptions{
		OutputDir:   outputDir,
		PackageName: "model",
		JSONTags:    true,
	})
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(outputDir, "sample_items.gen.go"))
	require.NoError(t, err)

	want, err := format.Source(got)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got), "generated file should already be gofmt-formatted")
	assert.Contains(t, string(got), "int64")
	assert.Contains(t, string(got), "// Database driver: sqlite")
}

func TestGenerateModelCode_includesGeneratedComment(t *testing.T) {
	code := GenerateModelCode(GenerateOptions{
		PackageName: "model",
		Driver:      constant.DriverSQLite,
	}, "SampleItem", "sample_items", []Column{
		{Name: "id", Type: "INTEGER", Nullable: false, PrimaryKey: true, AutoIncrement: true},
		{Name: "title", Type: "TEXT", Nullable: false},
	}, nil)

	assert.Contains(t, code, "// Code generated by app-tpl gen model. DO NOT EDIT.")
}

func TestGenerateModelCode_generatesTimestampHook(t *testing.T) {
	code := GenerateModelCode(GenerateOptions{
		PackageName: "model",
		Driver:      constant.DriverMySQL,
	}, "Videos", "videos", []Column{
		{Name: "id", Type: "bigint", Nullable: false, PrimaryKey: true, AutoIncrement: true, Unsigned: true},
		{Name: "title", Type: "varchar", Nullable: false},
		{Name: "created_at", Type: "datetime", Nullable: true},
		{Name: "updated_at", Type: "datetime", Nullable: true},
	}, nil)

	assert.Contains(t, code, `"context"`)
	assert.Contains(t, code, "var _ bun.BeforeAppendModelHook = (*Videos)(nil)")
	assert.Contains(t, code, "func (m *Videos) BeforeAppendModel(ctx context.Context, query bun.Query) error {")
	assert.Contains(t, code, "case *bun.InsertQuery:")
	assert.Contains(t, code, "m.CreatedAt = &now")
	assert.Contains(t, code, "case *bun.UpdateQuery:")
	assert.Contains(t, code, "m.UpdatedAt = &now")
}

func TestGenerateModelCode_generatesHookForNonNullableTimestamps(t *testing.T) {
	code := GenerateModelCode(GenerateOptions{
		PackageName: "model",
		Driver:      constant.DriverMySQL,
	}, "Banners", "banners", []Column{
		{Name: "id", Type: "bigint", Nullable: false, PrimaryKey: true, AutoIncrement: true, Unsigned: true},
		{Name: "created_at", Type: "datetime", Nullable: false},
		{Name: "updated_at", Type: "datetime", Nullable: false},
	}, nil)

	assert.Contains(t, code, "m.CreatedAt = now")
	assert.Contains(t, code, "m.UpdatedAt = now")
	assert.NotContains(t, code, "m.CreatedAt = &now")
	assert.NotContains(t, code, "m.UpdatedAt = &now")
}

func TestGenerateModelCode_generatesHookForCreatedAtOnly(t *testing.T) {
	code := GenerateModelCode(GenerateOptions{
		PackageName: "model",
		Driver:      constant.DriverMySQL,
	}, "CollectLogs", "collect_logs", []Column{
		{Name: "id", Type: "bigint", Nullable: false, PrimaryKey: true, AutoIncrement: true, Unsigned: true},
		{Name: "created_at", Type: "datetime", Nullable: true},
	}, nil)

	assert.Contains(t, code, "case *bun.InsertQuery:")
	assert.Contains(t, code, "m.CreatedAt = &now")
	assert.NotContains(t, code, "case *bun.UpdateQuery:")
}

func TestGenerateModelCode_noHookWithoutTimestampColumns(t *testing.T) {
	code := GenerateModelCode(GenerateOptions{
		PackageName: "model",
		Driver:      constant.DriverMySQL,
	}, "VideoActors", "video_actors", []Column{
		{Name: "id", Type: "bigint", Nullable: false, PrimaryKey: true, AutoIncrement: true, Unsigned: true},
		{Name: "video_id", Type: "bigint", Nullable: false, Unsigned: true},
		{Name: "actor_id", Type: "bigint", Nullable: false, Unsigned: true},
	}, nil)

	assert.NotContains(t, code, "BeforeAppendModelHook")
	assert.NotContains(t, code, "BeforeAppendModel")
	assert.NotContains(t, code, `"context"`)
}

func TestGenerateModelCode_addsSoftDeleteTag(t *testing.T) {
	code := GenerateModelCode(GenerateOptions{
		PackageName: "model",
		Driver:      constant.DriverMySQL,
		JSONTags:    true,
	}, "Users", "users", []Column{
		{Name: "id", Type: "bigint", Nullable: false, PrimaryKey: true, AutoIncrement: true},
		{Name: "email", Type: "varchar", Nullable: false},
		{Name: "created_at", Type: "datetime", Nullable: false},
		{Name: "updated_at", Type: "datetime", Nullable: false},
		{Name: "deleted_at", Type: "datetime", Nullable: true},
	}, nil)

	assert.Contains(t, code, "\tDeletedAt *time.Time `bun:\"deleted_at,soft_delete,nullzero\" json:\"deleted_at\"`")
	assert.Contains(t, code, "var _ bun.BeforeAppendModelHook = (*Users)(nil)")
	assert.Contains(t, code, "case *bun.InsertQuery:")
	assert.Contains(t, code, "case *bun.UpdateQuery:")
	assert.NotContains(t, code, "m.DeletedAt")
}

func TestGenerateModelCode_noSoftDeleteWithoutDeletedAt(t *testing.T) {
	code := GenerateModelCode(GenerateOptions{
		PackageName: "model",
		Driver:      constant.DriverSQLite,
		JSONTags:    true,
	}, "SampleItem", "sample_items", []Column{
		{Name: "id", Type: "INTEGER", Nullable: false, PrimaryKey: true, AutoIncrement: true},
		{Name: "title", Type: "TEXT", Nullable: false},
	}, nil)

	assert.NotContains(t, code, "soft_delete")
	assert.NotContains(t, code, "DeletedAt")
}

func TestGenerateModelCode_noSoftDeleteForNonNullableDeletedAt(t *testing.T) {
	code := GenerateModelCode(GenerateOptions{
		PackageName: "model",
		Driver:      constant.DriverMySQL,
		JSONTags:    true,
	}, "Users", "users", []Column{
		{Name: "id", Type: "bigint", Nullable: false, PrimaryKey: true, AutoIncrement: true},
		{Name: "deleted_at", Type: "datetime", Nullable: false},
	}, nil)

	assert.Contains(t, code, "\tDeletedAt time.Time `bun:\"deleted_at,notnull\" json:\"deleted_at\"`")
	assert.NotContains(t, code, "soft_delete")
}

func TestGenerateModelCode_writesDefaultTag(t *testing.T) {
	code := GenerateModelCode(GenerateOptions{
		PackageName: "model",
		Driver:      constant.DriverSQLite,
		JSONTags:    true,
	}, "Items", "items", []Column{
		{Name: "id", Type: "INTEGER", PrimaryKey: true, AutoIncrement: true},
		{Name: "status", Type: "INTEGER", Default: "1"},
		{Name: "created_at", Type: "DATETIME", Default: "CURRENT_TIMESTAMP"},
	}, nil)

	assert.Contains(t, code, `default:1`)
	assert.Contains(t, code, `default:current_timestamp`)
}

func TestGenerateModels_dryRunDoesNotWriteFiles(t *testing.T) {
	db := testutil.OpenBunDB(t)
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `CREATE TABLE sample_items (id INTEGER PRIMARY KEY, title TEXT NOT NULL)`)
	require.NoError(t, err)

	outputDir := t.TempDir()
	var buf bytes.Buffer
	err = GenerateModels(ctx, db, constant.DriverSQLite, []string{"sample_items"}, GenerateOptions{
		OutputDir:   outputDir,
		PackageName: "model",
		JSONTags:    true,
		DryRun:      true,
		Out:         &buf,
	})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "// file: sample_items.gen.go")
	assert.Contains(t, buf.String(), "type SampleItems struct")
	_, err = os.Stat(filepath.Join(outputDir, "sample_items.gen.go"))
	assert.Error(t, err)
}

func TestGenerateModels_excludeSkipsWrite(t *testing.T) {
	db := testutil.OpenBunDB(t)
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `CREATE TABLE categories (id INTEGER PRIMARY KEY)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE TABLE videos (id INTEGER PRIMARY KEY, category_id INTEGER NOT NULL)`)
	require.NoError(t, err)

	outputDir := t.TempDir()
	err = GenerateModels(ctx, db, constant.DriverSQLite, []string{"videos"}, GenerateOptions{
		OutputDir:   outputDir,
		PackageName: "model",
		JSONTags:    true,
		Exclude:     map[string]struct{}{"categories": {}},
		Relations: []Relation{{
			SourceTable: "videos", SourceColumn: "category_id",
			TargetTable: "categories", TargetColumn: "id",
		}},
	})
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(outputDir, "videos.gen.go"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(outputDir, "categories.gen.go"))
	assert.Error(t, err)
}
