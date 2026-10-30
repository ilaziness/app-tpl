package gen

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/jinzhu/inflection"
	"gopkg.in/yaml.v3"
)

const RelationManyToMany = "many-to-many"

var goIdentifierRE = regexp.MustCompile(`^[A-Z][a-zA-Z0-9_]*$`)

// Relation describes a logical relation from a source table column to a target table column.
type Relation struct {
	Type         string // "" (belongs-to/has-many) or "many-to-many"
	Through      string // junction table for many-to-many
	SourceTable  string
	SourceColumn string // may be "a+b" for composite
	TargetTable  string
	TargetColumn string
	Field        string
	ReverseField string
	HasOne       bool   // reverse side is has-one instead of has-many
	M2MJoin      string // "LeftGoField=RightGoField" for Bun m2m tag
	SkipReverse  bool   // true when this FK is absorbed into an m2m (no has-many on ends)
}

type relationsFile struct {
	Relations []Relation `yaml:"relations"`
}

// LoadRelations reads logical relations from a YAML file.
func LoadRelations(path string) ([]Relation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read relations file: %w", err)
	}

	var file relationsFile
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("parse relations file: %w", err)
	}
	if err := validateRelations(file.Relations); err != nil {
		return nil, err
	}
	return file.Relations, nil
}

// LoadRelationsOptional loads relations. When required is false, a missing file yields an empty list.
func LoadRelationsOptional(path string, required bool) ([]Relation, error) {
	relations, err := LoadRelations(path)
	if err != nil {
		if !required && errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return relations, nil
}

func (r *Relation) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		parts := strings.Split(value.Value, "->")
		if len(parts) != 2 {
			return fmt.Errorf("invalid relation %q: expected source.table_column -> target.table_column", value.Value)
		}
		var err error
		r.SourceTable, r.SourceColumn, err = parseRelationEndpoint(strings.TrimSpace(parts[0]))
		if err != nil {
			return err
		}
		r.TargetTable, r.TargetColumn, err = parseRelationEndpoint(strings.TrimSpace(parts[1]))
		return err
	case yaml.MappingNode:
		allowedFields := map[string]struct{}{
			"source":        {},
			"target":        {},
			"field":         {},
			"reverse_field": {},
			"type":          {},
			"through":       {},
		}
		seenFields := make(map[string]struct{}, len(value.Content)/2)
		for i := 0; i < len(value.Content); i += 2 {
			field := value.Content[i].Value
			if _, ok := allowedFields[field]; !ok {
				return fmt.Errorf("unknown relation field %q", field)
			}
			if _, ok := seenFields[field]; ok {
				return fmt.Errorf("duplicate relation field %q", field)
			}
			seenFields[field] = struct{}{}
		}

		var raw struct {
			Source       string `yaml:"source"`
			Target       string `yaml:"target"`
			Field        string `yaml:"field"`
			ReverseField string `yaml:"reverse_field"`
			Type         string `yaml:"type"`
			Through      string `yaml:"through"`
		}
		if err := value.Decode(&raw); err != nil {
			return err
		}
		r.Type = raw.Type
		r.Through = raw.Through
		r.Field = raw.Field
		r.ReverseField = raw.ReverseField

		var err error
		r.SourceTable, r.SourceColumn, err = parseRelationEndpoint(raw.Source)
		if err != nil {
			return err
		}
		r.TargetTable, r.TargetColumn, err = parseRelationEndpoint(raw.Target)
		if err != nil {
			return err
		}
		if r.Type == RelationManyToMany {
			if r.Through == "" {
				return fmt.Errorf("many-to-many relation requires through")
			}
			if err := validateTableName(r.Through); err != nil {
				return fmt.Errorf("invalid through table: %w", err)
			}
		} else if r.Type != "" && r.Type != RelationManyToMany {
			return fmt.Errorf("unsupported relation type %q", r.Type)
		}
		return nil
	default:
		return fmt.Errorf("invalid relation: expected a string or mapping")
	}
}

func parseRelationEndpoint(value string) (string, string, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid relation endpoint %q: expected table.column", value)
	}
	if err := validateTableName(parts[0]); err != nil {
		return "", "", err
	}
	cols := splitCompositeColumns(parts[1])
	for _, col := range cols {
		if err := validateTableName(col); err != nil {
			return "", "", fmt.Errorf("invalid column name %q", col)
		}
	}
	return parts[0], strings.Join(cols, "+"), nil
}

func splitCompositeColumns(value string) []string {
	parts := strings.Split(value, "+")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func validateRelations(relations []Relation) error {
	seen := make(map[string]struct{}, len(relations))
	for _, relation := range relations {
		if err := validateRelation(relation); err != nil {
			return err
		}
		key := relation.key()
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate relation %s", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateRelation(relation Relation) error {
	for _, name := range []string{relation.SourceTable, relation.TargetTable} {
		if err := validateTableName(name); err != nil {
			return fmt.Errorf("invalid relation: %w", err)
		}
	}
	for _, col := range splitCompositeColumns(relation.SourceColumn) {
		if err := validateTableName(col); err != nil {
			return fmt.Errorf("invalid relation source column %q", col)
		}
	}
	for _, col := range splitCompositeColumns(relation.TargetColumn) {
		if err := validateTableName(col); err != nil {
			return fmt.Errorf("invalid relation target column %q", col)
		}
	}
	if relation.Type == RelationManyToMany {
		if err := validateTableName(relation.Through); err != nil {
			return fmt.Errorf("invalid through table: %w", err)
		}
	}
	for _, name := range []string{relation.Field, relation.ReverseField} {
		if name != "" && !goIdentifierRE.MatchString(name) {
			return fmt.Errorf("invalid relation field %q", name)
		}
	}
	return nil
}

func (r Relation) key() string {
	if r.Type == RelationManyToMany {
		return "m2m:" + r.Through + ":" + r.SourceTable + "." + r.SourceColumn + "->" + r.TargetTable + "." + r.TargetColumn
	}
	return r.SourceTable + "." + r.SourceColumn + "->" + r.TargetTable + "." + r.TargetColumn
}

func relationFieldNames(relation Relation) (string, string) {
	field := relation.Field
	reverseField := relation.ReverseField

	if relation.Type == RelationManyToMany {
		if field == "" {
			field = inflection.Plural(ToCamelCase(relation.TargetTable))
		}
		if reverseField == "" {
			reverseField = inflection.Plural(ToCamelCase(relation.SourceTable))
		}
		return field, reverseField
	}

	self := relation.SourceTable == relation.TargetTable
	if field == "" {
		if self {
			field = "Parent"
		} else {
			field = inflection.Singular(ToCamelCase(relation.TargetTable))
		}
	}
	if reverseField == "" {
		if self {
			if relation.HasOne {
				reverseField = "Child"
			} else {
				reverseField = "Children"
			}
		} else {
			reverseField = inflection.Plural(ToCamelCase(relation.SourceTable))
		}
	}
	return field, reverseField
}

func mergeRelations(physical, logical []Relation) ([]Relation, error) {
	merged := make(map[string]Relation, len(physical)+len(logical))
	for _, relation := range physical {
		merged[relation.key()] = relation
	}
	for _, relation := range logical {
		key := relation.key()
		if existing, ok := merged[key]; ok {
			if relation.Field != "" {
				existing.Field = relation.Field
			}
			if relation.ReverseField != "" {
				existing.ReverseField = relation.ReverseField
			}
			if relation.Type != "" {
				existing.Type = relation.Type
			}
			if relation.Through != "" {
				existing.Through = relation.Through
			}
			merged[key] = existing
			continue
		}
		merged[key] = relation
	}

	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]Relation, 0, len(merged))
	fields := make(map[string]string, len(merged)*2)
	for _, key := range keys {
		relation := merged[key]
		if err := validateRelation(relation); err != nil {
			return nil, err
		}
		field, reverseField := relationFieldNames(relation)
		if relation.SourceTable == relation.TargetTable && field == reverseField {
			return nil, fmt.Errorf("relation field conflict on %s.%s", relation.SourceTable, field)
		}
		occupy := map[string]string{relation.SourceTable + "." + field: relation.key()}
		if relation.Type == RelationManyToMany || !relation.SkipReverse {
			occupy[relation.TargetTable+"."+reverseField] = relation.key()
		}
		for key, name := range occupy {
			if existing, ok := fields[key]; ok && existing != name {
				return nil, fmt.Errorf("relation field conflict on %s between %s and %s", key, existing, name)
			}
			fields[key] = name
		}
		result = append(result, relation)
	}
	return result, nil
}

// annotateHasOne marks relations whose source FK is the whole single-column PK
// or a single-column unique constraint (composite PK columns are not has-one).
func annotateHasOne(relations []Relation, uniqueSingleCols map[string]map[string]bool, pkCols map[string]map[string]bool) {
	for i := range relations {
		rel := &relations[i]
		if rel.Type == RelationManyToMany {
			continue
		}
		cols := splitCompositeColumns(rel.SourceColumn)
		if len(cols) != 1 {
			continue
		}
		col := cols[0]
		if uniqueSingleCols[rel.SourceTable][col] {
			rel.HasOne = true
			continue
		}
		pks := pkCols[rel.SourceTable]
		if len(pks) == 1 && pks[col] {
			rel.HasOne = true
		}
	}
}

var junctionMetaColumns = map[string]struct{}{
	"created_at": {},
	"updated_at": {},
	"deleted_at": {},
}

// detectPureM2MTables finds junction tables and returns m2m relations plus FK keys to suppress reverse has-many.
func detectPureM2MTables(
	tableColumns map[string][]Column,
	relations []Relation,
) (m2m []Relation, suppressReverse map[string]struct{}) {
	suppressReverse = make(map[string]struct{})

	// Group belongs-to FKs by source table.
	fksByTable := make(map[string][]Relation)
	for _, rel := range relations {
		if rel.Type == RelationManyToMany {
			continue
		}
		fksByTable[rel.SourceTable] = append(fksByTable[rel.SourceTable], rel)
	}

	for table, fks := range fksByTable {
		if len(fks) != 2 {
			continue
		}
		if fks[0].TargetTable == fks[1].TargetTable {
			continue
		}
		cols := tableColumns[table]
		if cols == nil {
			continue
		}

		fkColSet := map[string]struct{}{}
		for _, fk := range fks {
			for _, c := range splitCompositeColumns(fk.SourceColumn) {
				fkColSet[c] = struct{}{}
			}
		}
		hasExtra := false
		for _, col := range cols {
			if _, isFK := fkColSet[col.Name]; isFK {
				continue
			}
			if _, meta := junctionMetaColumns[col.Name]; meta {
				continue
			}
			hasExtra = true
			break
		}
		if hasExtra || !fkColumnsAreCompleteKey(cols, fkColSet) {
			continue
		}

		left, right := fks[0], fks[1]
		// Stable order by target table name.
		if left.TargetTable > right.TargetTable {
			left, right = right, left
		}
		leftField := inflection.Singular(ToCamelCase(left.TargetTable))
		rightField := inflection.Singular(ToCamelCase(right.TargetTable))
		m2m = append(m2m, Relation{
			Type:         RelationManyToMany,
			Through:      table,
			SourceTable:  left.TargetTable,
			SourceColumn: left.TargetColumn,
			TargetTable:  right.TargetTable,
			TargetColumn: right.TargetColumn,
			M2MJoin:      leftField + "=" + rightField,
		})
		suppressReverse[left.key()] = struct{}{}
		suppressReverse[right.key()] = struct{}{}
	}
	return m2m, suppressReverse
}

func fkColumnsAreCompleteKey(columns []Column, fkCols map[string]struct{}) bool {
	pkSet := map[string]struct{}{}
	uniqueGroups := map[string]map[string]struct{}{}
	for _, col := range columns {
		if col.PrimaryKey {
			pkSet[col.Name] = struct{}{}
		}
		if col.Unique {
			g := col.UniqueGroup
			if g == "" {
				g = "__single__:" + col.Name
			}
			if uniqueGroups[g] == nil {
				uniqueGroups[g] = map[string]struct{}{}
			}
			uniqueGroups[g][col.Name] = struct{}{}
		}
	}
	if sameStringSet(pkSet, fkCols) {
		return true
	}
	for _, g := range uniqueGroups {
		if sameStringSet(g, fkCols) {
			return true
		}
	}
	return false
}

func sameStringSet(a, b map[string]struct{}) bool {
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// resolveM2MJoins fills M2MJoin for explicit many-to-many relations using FK relations from the junction.
func resolveM2MJoins(relations []Relation) ([]Relation, error) {
	fksByTable := make(map[string][]Relation)
	for _, rel := range relations {
		if rel.Type == RelationManyToMany {
			continue
		}
		fksByTable[rel.SourceTable] = append(fksByTable[rel.SourceTable], rel)
	}

	out := make([]Relation, 0, len(relations))
	for _, rel := range relations {
		if rel.Type != RelationManyToMany {
			out = append(out, rel)
			continue
		}
		if rel.M2MJoin != "" {
			out = append(out, rel)
			continue
		}
		fks := fksByTable[rel.Through]
		var leftFK, rightFK *Relation
		for i := range fks {
			fk := &fks[i]
			if fk.TargetTable == rel.SourceTable && columnsMatch(fk.TargetColumn, rel.SourceColumn) {
				leftFK = fk
			}
			if fk.TargetTable == rel.TargetTable && columnsMatch(fk.TargetColumn, rel.TargetColumn) {
				rightFK = fk
			}
		}
		if leftFK == nil || rightFK == nil {
			return nil, fmt.Errorf("many-to-many through %s: junction foreign keys to %s and %s not found",
				rel.Through, rel.SourceTable, rel.TargetTable)
		}
		leftField := leftFK.Field
		if leftField == "" {
			leftField = inflection.Singular(ToCamelCase(rel.SourceTable))
		}
		rightField := rightFK.Field
		if rightField == "" {
			rightField = inflection.Singular(ToCamelCase(rel.TargetTable))
		}
		rel.M2MJoin = leftField + "=" + rightField
		out = append(out, rel)
	}
	return out, nil
}

func columnsMatch(a, b string) bool {
	ca := splitCompositeColumns(a)
	cb := splitCompositeColumns(b)
	if len(ca) != len(cb) {
		return false
	}
	for i := range ca {
		if ca[i] != cb[i] {
			return false
		}
	}
	return true
}

func applySkipReverse(relations []Relation, suppress map[string]struct{}) []Relation {
	if len(suppress) == 0 {
		return relations
	}
	out := make([]Relation, len(relations))
	for i, rel := range relations {
		if _, ok := suppress[rel.key()]; ok {
			rel.SkipReverse = true
		}
		out[i] = rel
	}
	return out
}
