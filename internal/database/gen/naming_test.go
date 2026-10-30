package gen

import (
	"testing"

	"github.com/ilaziness/app-tpl/internal/constant"
	"github.com/stretchr/testify/assert"
)

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user_name", "UserName"},
		{"users", "Users"},
		{"order_item_id", "OrderItemID"},
		{"id", "ID"},
		{"play_url", "PlayURL"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, ToCamelCase(tt.input))
		})
	}
}

func TestMapColumn_MySQL(t *testing.T) {
	assert.Equal(t, "int64", MapColumn(constant.DriverMySQL, Column{Type: "bigint"}).GoType)
	assert.Equal(t, "*string", MapColumn(constant.DriverMySQL, Column{Type: "text", Nullable: true}).GoType)
	assert.Equal(t, "uint32", MapColumn(constant.DriverMySQL, Column{Type: "int", Unsigned: true}).GoType)
	assert.Equal(t, "bool", MapColumn(constant.DriverMySQL, Column{Type: "tinyint", ColumnType: "tinyint(1)"}).GoType)
	assert.Equal(t, "int8", MapColumn(constant.DriverMySQL, Column{Type: "tinyint", ColumnType: "tinyint(4)"}).GoType)
	assert.Equal(t, "int32", MapColumn(constant.DriverMySQL, Column{Type: "mediumint", ColumnType: "mediumint"}).GoType)
	assert.Equal(t, "float32", MapColumn(constant.DriverMySQL, Column{Type: "float"}).GoType)
	assert.Equal(t, "float64", MapColumn(constant.DriverMySQL, Column{Type: "double"}).GoType)
	assert.Equal(t, "string", MapColumn(constant.DriverMySQL, Column{Type: "decimal"}).GoType)
	assert.Equal(t, "[]byte", MapColumn(constant.DriverMySQL, Column{Type: "bit"}).GoType)
	ft := MapColumn(constant.DriverMySQL, Column{Type: "json"})
	assert.Equal(t, "json.RawMessage", ft.GoType)
	assert.Equal(t, "json", ft.BunType)
	enum := MapColumn(constant.DriverMySQL, Column{Type: "enum", ColumnType: "enum('Active','Inactive')"})
	assert.Equal(t, []string{"Active", "Inactive"}, enum.EnumValues)
}

func TestMapColumn_Postgres(t *testing.T) {
	assert.Equal(t, "int16", MapColumn(constant.DriverPostgreSQL, Column{Type: "smallint", UDTName: "int2"}).GoType)
	assert.Equal(t, "int32", MapColumn(constant.DriverPostgreSQL, Column{Type: "integer", UDTName: "int4"}).GoType)
	assert.Equal(t, "int64", MapColumn(constant.DriverPostgreSQL, Column{Type: "bigint", UDTName: "int8"}).GoType)
	assert.Equal(t, "float32", MapColumn(constant.DriverPostgreSQL, Column{Type: "real", UDTName: "float4"}).GoType)
	assert.Equal(t, "string", MapColumn(constant.DriverPostgreSQL, Column{Type: "numeric"}).GoType)
	jsonb := MapColumn(constant.DriverPostgreSQL, Column{Type: "jsonb", UDTName: "jsonb"})
	assert.Equal(t, "json.RawMessage", jsonb.GoType)
	assert.Equal(t, "jsonb", jsonb.BunType)
	arr := MapColumn(constant.DriverPostgreSQL, Column{Type: "ARRAY", UDTName: "_int4", ArrayDims: 1})
	assert.Equal(t, "[]int32", arr.GoType)
	assert.True(t, arr.Array)
	nullArr := MapColumn(constant.DriverPostgreSQL, Column{Type: "ARRAY", UDTName: "_text", ArrayDims: 1, Nullable: true})
	assert.Equal(t, "*[]string", nullArr.GoType)
}

func TestMapColumn_SQLite(t *testing.T) {
	assert.Equal(t, "int64", MapColumn(constant.DriverSQLite, Column{Type: "INTEGER"}).GoType)
	assert.Equal(t, "int64", MapColumn(constant.DriverSQLite, Column{Type: "INT"}).GoType)
	assert.Equal(t, "int64", MapColumn(constant.DriverSQLite, Column{Type: "BIGINT"}).GoType)
	assert.Equal(t, "bool", MapColumn(constant.DriverSQLite, Column{Type: "BOOLEAN"}).GoType)
	jsonFT := MapColumn(constant.DriverSQLite, Column{Type: "JSON"})
	assert.Equal(t, "json.RawMessage", jsonFT.GoType)
	assert.Equal(t, "json", jsonFT.BunType)
	varchar := MapColumn(constant.DriverSQLite, Column{Type: "VARCHAR(64)"})
	assert.Equal(t, "string", varchar.GoType)
	assert.Equal(t, 64, varchar.Length)
}

func TestNormalizeDefault(t *testing.T) {
	assert.Equal(t, "1", NormalizeDefault(constant.DriverMySQL, "1", "int32"))
	assert.Equal(t, "current_timestamp", NormalizeDefault(constant.DriverMySQL, "CURRENT_TIMESTAMP", "time.Time"))
	assert.Equal(t, "", NormalizeDefault(constant.DriverMySQL, "1", "bool"))
	assert.Equal(t, "'active'", NormalizeDefault(constant.DriverPostgreSQL, "'active'::text", "string"))
	assert.Equal(t, "now()", NormalizeDefault(constant.DriverPostgreSQL, "now()", "time.Time"))
	assert.Equal(t, "", NormalizeDefault(constant.DriverPostgreSQL, "nextval('users_id_seq'::regclass)", "int64"))
	assert.Equal(t, "current_timestamp", NormalizeDefault(constant.DriverSQLite, "CURRENT_TIMESTAMP", "time.Time"))
	assert.Equal(t, "", NormalizeDefault(constant.DriverSQLite, "(datetime('now'))", "time.Time"))
}

func TestUniqueAliases(t *testing.T) {
	aliases := UniqueAliases([]string{"users", "user_settings"})
	assert.NotEqual(t, aliases["users"], aliases["user_settings"])
	assert.NotEmpty(t, aliases["users"])
	assert.NotEmpty(t, aliases["user_settings"])
}

func TestSanitizeComment(t *testing.T) {
	assert.Equal(t, "hello world", SanitizeComment("hello\nworld"))
	assert.Equal(t, "a b", SanitizeComment("a */ b"))
}
