package gen

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/ilaziness/app-tpl/internal/constant"
)

var (
	charLengthRE  = regexp.MustCompile(`(?i)^(char|varchar)\((\d+)\)`)
	mysqlTinyInt1 = regexp.MustCompile(`(?i)^tinyint\s*\(\s*1\s*\)`)
	mysqlEnumRE   = regexp.MustCompile(`(?i)^(enum|set)\((.*)\)$`)
)

// FieldType describes the Go type and Bun options for a column.
type FieldType struct {
	GoType     string // e.g. "int64", "*string", "json.RawMessage", "*[]int32"
	BunType    string // e.g. "json", "jsonb"
	Array      bool
	Length     int
	EnumValues []string
	IsSet      bool   // MySQL SET type: keep values but skip oneof validation
	RawComment string // extra note when falling back to string
}

// MapColumn maps a column to a FieldType for the given driver.
func MapColumn(driver string, col Column) FieldType {
	switch driver {
	case constant.DriverMySQL:
		return mapMySQLColumn(col)
	case constant.DriverPostgres, constant.DriverPostgreSQL:
		return mapPostgresColumn(col)
	case constant.DriverSQLite, constant.DriverSQLite3:
		return mapSQLiteColumn(col)
	default:
		ft := FieldType{GoType: "string", RawComment: col.Type}
		return applyNullable(ft, col.Nullable)
	}
}

func applyNullable(ft FieldType, nullable bool) FieldType {
	if !nullable || ft.GoType == "" {
		return ft
	}
	if strings.HasPrefix(ft.GoType, "*") {
		return ft
	}
	// []byte is already a reference type; nullable stays []byte (nil slice).
	if ft.GoType == "[]byte" {
		return ft
	}
	if ft.Array {
		ft.GoType = "*[]" + strings.TrimPrefix(ft.GoType, "[]")
		return ft
	}
	ft.GoType = "*" + ft.GoType
	return ft
}

func applyUnsigned(goType string, unsigned bool) string {
	if !unsigned {
		return goType
	}
	switch goType {
	case "int8":
		return "uint8"
	case "int16":
		return "uint16"
	case "int32":
		return "uint32"
	case "int64":
		return "uint64"
	default:
		return goType
	}
}

func mapMySQLColumn(col Column) FieldType {
	dataType := strings.ToLower(strings.TrimSpace(col.Type))
	columnType := strings.ToLower(strings.TrimSpace(col.ColumnType))
	if columnType == "" {
		columnType = dataType
	}

	ft := FieldType{}

	if mysqlTinyInt1.MatchString(columnType) {
		ft.GoType = "bool"
		return applyNullable(ft, col.Nullable)
	}

	switch dataType {
	case "tinyint":
		ft.GoType = applyUnsigned("int8", col.Unsigned)
	case "smallint", "year":
		ft.GoType = applyUnsigned("int16", col.Unsigned)
	case "mediumint", "int", "integer":
		ft.GoType = applyUnsigned("int32", col.Unsigned)
	case "bigint":
		ft.GoType = applyUnsigned("int64", col.Unsigned)
	case "float":
		ft.GoType = "float32"
	case "double", "double precision":
		ft.GoType = "float64"
	case "decimal", "numeric":
		ft.GoType = "string"
	case "bit":
		ft.GoType = "[]byte"
	case "char", "varchar":
		ft.GoType = "string"
		if m := charLengthRE.FindStringSubmatch(columnType); len(m) == 3 {
			ft.Length, _ = strconv.Atoi(m[2])
		}
	case "text", "tinytext", "mediumtext", "longtext":
		ft.GoType = "string"
	case "json":
		ft.GoType = "json.RawMessage"
		ft.BunType = "json"
	case "blob", "tinyblob", "mediumblob", "longblob", "binary", "varbinary":
		ft.GoType = "[]byte"
	case "enum":
		ft.GoType = "string"
		ft.EnumValues = parseMySQLEnumOrSet(col.ColumnType)
	case "set":
		ft.GoType = "string"
		ft.EnumValues = parseMySQLEnumOrSet(col.ColumnType)
		ft.IsSet = true
	case "date", "time", "datetime", "timestamp":
		ft.GoType = "time.Time"
	case "boolean", "bool":
		ft.GoType = "bool"
	default:
		ft.GoType = "string"
		ft.RawComment = col.Type
	}

	return applyNullable(ft, col.Nullable)
}

func parseMySQLEnumOrSet(columnType string) []string {
	m := mysqlEnumRE.FindStringSubmatch(columnType)
	if m == nil {
		return nil
	}
	return splitQuotedList(m[2])
}

func splitQuotedList(s string) []string {
	var values []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\'' && !inQuote:
			inQuote = true
		case c == '\'' && inQuote:
			if i+1 < len(s) && s[i+1] == '\'' {
				cur.WriteByte('\'')
				i++
				continue
			}
			inQuote = false
			values = append(values, cur.String())
			cur.Reset()
		case inQuote:
			cur.WriteByte(c)
		}
	}
	return values
}

func mapPostgresColumn(col Column) FieldType {
	dataType := strings.ToLower(strings.TrimSpace(col.Type))
	udt := strings.ToLower(strings.TrimSpace(col.UDTName))

	ft := FieldType{}

	if col.ArrayDims > 1 {
		ft.GoType = "string"
		ft.RawComment = col.Type
		if udt != "" {
			ft.RawComment = udt
		}
		return applyNullable(ft, col.Nullable)
	}

	if col.ArrayDims == 1 || dataType == "array" || strings.HasPrefix(udt, "_") {
		elem := strings.TrimPrefix(udt, "_")
		if elem == "" {
			elem = strings.TrimSuffix(dataType, "[]")
		}
		elemFT := mapPostgresScalar(elem, elem)
		switch elemFT.GoType {
		case "int16", "int32", "int64", "float32", "float64", "string", "bool", "time.Time":
			if elemFT.RawComment != "" {
				// Unknown/custom element type fell back to string with a note — keep as plain string.
				ft.GoType = "string"
				ft.RawComment = col.Type
				return applyNullable(ft, col.Nullable)
			}
			ft.GoType = "[]" + elemFT.GoType
			ft.Array = true
			return applyNullable(ft, col.Nullable)
		default:
			ft.GoType = "string"
			ft.RawComment = col.Type
			return applyNullable(ft, col.Nullable)
		}
	}

	ft = mapPostgresScalar(dataType, udt)
	if len(col.EnumValues) > 0 {
		ft.EnumValues = append([]string(nil), col.EnumValues...)
	}
	if col.Length > 0 && ft.GoType == "string" {
		ft.Length = col.Length
	}
	return applyNullable(ft, col.Nullable)
}

func mapPostgresScalar(dataType, udt string) FieldType {
	ft := FieldType{}
	name := dataType
	if name == "user-defined" || name == "array" {
		name = udt
	}
	name = strings.ToLower(strings.TrimSpace(name))

	switch name {
	case "boolean", "bool":
		ft.GoType = "bool"
	case "smallint", "int2":
		ft.GoType = "int16"
	case "integer", "int", "int4":
		ft.GoType = "int32"
	case "bigint", "int8":
		ft.GoType = "int64"
	case "real", "float4":
		ft.GoType = "float32"
	case "double precision", "float8":
		ft.GoType = "float64"
	case "numeric", "decimal":
		ft.GoType = "string"
	case "character varying", "varchar", "character", "char", "text", "citext":
		ft.GoType = "string"
		// Length is applied by caller via col.Length when available.
	case "json":
		ft.GoType = "json.RawMessage"
		ft.BunType = "json"
	case "jsonb":
		ft.GoType = "json.RawMessage"
		ft.BunType = "jsonb"
	case "bytea":
		ft.GoType = "[]byte"
	case "uuid", "inet", "cidr", "interval", "money":
		ft.GoType = "string"
	case "date", "time", "timetz", "timestamp", "timestamptz",
		"time without time zone", "time with time zone",
		"timestamp without time zone", "timestamp with time zone":
		ft.GoType = "time.Time"
	default:
		ft.GoType = "string"
		if name != "" {
			ft.RawComment = name
		}
	}
	return ft
}

func mapSQLiteColumn(col Column) FieldType {
	decl := strings.TrimSpace(col.Type)
	upper := strings.ToUpper(decl)
	// Strip length for base type matching: VARCHAR(255) -> VARCHAR
	base := upper
	if i := strings.IndexByte(base, '('); i >= 0 {
		base = strings.TrimSpace(base[:i])
	}

	ft := FieldType{}
	switch base {
	case "INTEGER", "INT", "BIGINT", "TINYINT", "SMALLINT", "MEDIUMINT", "INT2", "INT8":
		ft.GoType = "int64"
	case "REAL", "FLOAT", "DOUBLE", "DOUBLE PRECISION":
		ft.GoType = "float64"
	case "NUMERIC", "DECIMAL":
		ft.GoType = "string"
	case "TEXT", "CLOB", "CHAR", "VARCHAR", "NVARCHAR", "VARYING CHARACTER", "NATIVE CHARACTER":
		ft.GoType = "string"
		if m := charLengthRE.FindStringSubmatch(decl); len(m) == 3 {
			ft.Length, _ = strconv.Atoi(m[2])
		}
	case "BOOLEAN", "BOOL":
		ft.GoType = "bool"
	case "BLOB":
		ft.GoType = "[]byte"
	case "JSON":
		ft.GoType = "json.RawMessage"
		ft.BunType = "json"
	case "DATE", "TIME", "DATETIME", "TIMESTAMP":
		ft.GoType = "time.Time"
	default:
		ft.GoType = "string"
		if decl != "" {
			ft.RawComment = decl
		}
	}
	return applyNullable(ft, col.Nullable)
}
