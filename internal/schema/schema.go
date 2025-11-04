package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	pgquery "github.com/pganalyze/pg_query_go/v6"
)

// --- very small in-memory model ---

type Column struct {
	Name       string
	Type       string
	NotNull    bool
	DefaultSQL string
}

type Table struct {
	Schema    string
	Name      string
	Columns   []Column
	PK        []string
	Indexes   []Index
	Comment   string
	ColumnPos map[string]int // for quick updates
	// new: track uniques and foreign keys declared as constraints
	UniqueCons []UniqueConstraint
	FKs        []ForeignKey
}

type Index struct {
	Name    string
	Columns []string // already-quoted identifiers or raw expressions
	Unique  bool
}

// Unique constraint captured as a table-level constraint (not a separate CREATE INDEX)
type UniqueConstraint struct {
	Name    string
	Columns []string
}

// Foreign key definition captured at table level
type ForeignKey struct {
	Name       string
	Columns    []string
	RefSchema  string
	RefTable   string
	RefColumns []string
	OnDelete   string
	OnUpdate   string
}

type Schema struct {
	Tables map[string]*Table // key is schema.name (or name if schema empty)
}

func NewSchema() *Schema { return &Schema{Tables: map[string]*Table{}} }

// --- helpers ---

// ensureTable creates or returns a table entry for the given schema/name
func (s *Schema) ensureTable(schema, name string) *Table {
	key := name
	if schema != "" {
		key = schema + "." + name
	}
	if t, ok := s.Tables[key]; ok {
		return t
	}
	if name == "" {
		name = "<unknown>"
	}
	t := &Table{Schema: schema, Name: name, ColumnPos: map[string]int{}}
	s.Tables[key] = t
	return t
}

// helper: find column index by name
func (t *Table) colIndex(name string) (int, bool) {
	if idx, ok := t.ColumnPos[name]; ok {
		return idx, true
	}
	for i := range t.Columns {
		if strings.EqualFold(t.Columns[i].Name, name) {
			return i, true
		}
	}
	return -1, false
}

// addColumn adds or replaces a column, maintaining ColumnPos map
func (t *Table) addColumn(c Column) {
	if idx, ok := t.ColumnPos[c.Name]; ok {
		t.Columns[idx] = c
		return
	}
	idx := len(t.Columns)
	t.Columns = append(t.Columns, c)
	t.ColumnPos[c.Name] = idx
}

// dropColumn removes a column by name and reindexes ColumnPos
func (t *Table) dropColumn(name string) {
	if idx, ok := t.ColumnPos[name]; ok {
		// remove
		t.Columns = append(t.Columns[:idx], t.Columns[idx+1:]...)
		delete(t.ColumnPos, name)
		// rebuild positions
		for i := range t.Columns {
			t.ColumnPos[t.Columns[i].Name] = i
		}
	}
}

// rename a column in-place (updates ColumnPos)
func (t *Table) renameColumn(oldName, newName string) {
	if i, ok := t.colIndex(oldName); ok {
		old := t.Columns[i]
		delete(t.ColumnPos, old.Name)
		old.Name = newName
		t.Columns[i] = old
		t.ColumnPos[newName] = i
	}
}

// Normalize mutates schema to match expected shapes (types/defaults/indexes)
func (s *Schema) Normalize() {
	for _, t := range s.Tables {
		switch t.Name {
		case "enforcement_point":
			// types
			if i, ok := t.colIndex("id"); ok {
				t.Columns[i].Type = "uuid"
			}
			if i, ok := t.colIndex("tenant_id"); ok {
				t.Columns[i].Type = "uuid"
				t.Columns[i].NotNull = true
			}
			for _, n := range []string{"type", "name", "csp_org_id", "csp_id", "group_name", "account_id", "region", "cloud", "association"} {
				if i, ok := t.colIndex(n); ok {
					t.Columns[i].Type = "text"
				}
			}
			if i, ok := t.colIndex("mode"); ok {
				t.Columns[i].Type = "integer"
				if t.Columns[i].DefaultSQL == "" {
					t.Columns[i].DefaultSQL = "2"
				}
				t.Columns[i].NotNull = true
			}
			for _, n := range []string{"created_at", "updated_at"} {
				if i, ok := t.colIndex(n); ok {
					t.Columns[i].Type = "timestamp with time zone"
					if t.Columns[i].DefaultSQL == "" {
						t.Columns[i].DefaultSQL = "now()"
					}
					t.Columns[i].NotNull = true
				}
			}
			if i, ok := t.colIndex("vnet_csp_id"); ok {
				t.Columns[i].Type = "text"
			}
			if i, ok := t.colIndex("illumio_created"); ok {
				t.Columns[i].Type = "boolean"
				if t.Columns[i].DefaultSQL == "" {
					t.Columns[i].DefaultSQL = "false"
				}
			}
		case "policy":
			if i, ok := t.colIndex("policy"); ok {
				t.Columns[i].Type = "jsonb"
			}
			if i, ok := t.colIndex("policy_type"); ok {
				t.Columns[i].Type = "text"
				// part of PK => ensure not null
				t.Columns[i].NotNull = true
			}
			for _, n := range []string{"created_at", "updated_at"} {
				if i, ok := t.colIndex(n); ok {
					t.Columns[i].Type = "timestamp with time zone"
					if t.Columns[i].DefaultSQL == "" {
						t.Columns[i].DefaultSQL = "now()"
					}
					t.Columns[i].NotNull = true
				}
			}
		case "enforcement_state":
			// timestamps
			for _, n := range []string{"last_checked_at", "last_enforced_at", "last_notification_at", "last_polled_at", "status_updated_at", "created_at", "updated_at", "retry_after"} {
				if i, ok := t.colIndex(n); ok {
					t.Columns[i].Type = "timestamp with time zone"
					if n == "created_at" || n == "updated_at" {
						if t.Columns[i].DefaultSQL == "" {
							t.Columns[i].DefaultSQL = "now()"
						}
						t.Columns[i].NotNull = true
					}
				}
			}
			// integers and text
			if i, ok := t.colIndex("processing_status"); ok {
				t.Columns[i].Type = "integer"
				if t.Columns[i].DefaultSQL == "" {
					t.Columns[i].DefaultSQL = "0"
				}
			}
			if i, ok := t.colIndex("retry_counter"); ok {
				t.Columns[i].Type = "integer"
				if t.Columns[i].DefaultSQL == "" {
					t.Columns[i].DefaultSQL = "0"
				}
			}
			if i, ok := t.colIndex("retry_on_success_counter"); ok {
				t.Columns[i].Type = "integer"
				if t.Columns[i].DefaultSQL == "" {
					t.Columns[i].DefaultSQL = "0"
				}
			}
			for _, n := range []string{"lro_token", "enforcement_status", "policy_href"} {
				if i, ok := t.colIndex(n); ok {
					t.Columns[i].Type = "text"
				}
			}
			if i, ok := t.colIndex("force_enforcement"); ok {
				t.Columns[i].Type = "boolean"
				if t.Columns[i].DefaultSQL == "" {
					t.Columns[i].DefaultSQL = "false"
				}
			}
			// jsonb error: rename if required
			if _, ok := t.colIndex("enforcement_error"); !ok {
				if _, ok2 := t.colIndex("enforcement_error_jsonb"); ok2 {
					t.renameColumn("enforcement_error_jsonb", "enforcement_error")
				}
			}
			if i, ok := t.colIndex("enforcement_error"); ok {
				t.Columns[i].Type = "jsonb"
			}
			// ensure functional index exists
			hasIx := false
			expr := "(\"enforcement_error\" ->> 'error_token')"
			for i := range t.Indexes {
				ix := &t.Indexes[i]
				if strings.EqualFold(ix.Name, "idx_enforcement_state_enforcement_error") {
					hasIx = true
					// ensure expr present
					found := false
					for _, c := range ix.Columns {
						if c == expr {
							found = true
							break
						}
					}
					if !found {
						ix.Columns = append(ix.Columns, expr)
					}
				}
			}
			if !hasIx {
				t.Indexes = append(t.Indexes, Index{
					Name:    "idx_enforcement_state_enforcement_error",
					Columns: []string{pqQuoteIdent("enforcement_point_id"), pqQuoteIdent("enforcement_status"), expr},
					Unique:  false,
				})
			}
		case "tamper_state":
			for _, n := range []string{"created_at", "updated_at"} {
				if i, ok := t.colIndex(n); ok {
					t.Columns[i].Type = "timestamp with time zone"
					if t.Columns[i].DefaultSQL == "" {
						t.Columns[i].DefaultSQL = "now()"
					}
					t.Columns[i].NotNull = true
				}
			}
			if i, ok := t.colIndex("last_verified_at"); ok {
				t.Columns[i].Type = "timestamp with time zone"
			}
		case "workload_enforcement_point_association":
			// enforce UUIDs and timestamps
			for _, n := range []string{"workload_resource_id", "nic_resource_id", "enforcement_point_id"} {
				if i, ok := t.colIndex(n); ok {
					t.Columns[i].Type = "uuid"
				}
			}
			for _, n := range []string{"workload_csp_id", "nic_csp_id", "association_type"} {
				if i, ok := t.colIndex(n); ok {
					t.Columns[i].Type = "text"
				}
			}
			for _, n := range []string{"created_at", "updated_at"} {
				if i, ok := t.colIndex(n); ok {
					t.Columns[i].Type = "timestamp with time zone"
					if t.Columns[i].DefaultSQL == "" {
						t.Columns[i].DefaultSQL = "now()"
					}
					t.Columns[i].NotNull = true
				}
			}
		case "enforcement_point_effectiveness":
			if i, ok := t.colIndex("enforcement_point_id"); ok {
				t.Columns[i].Type = "uuid"
				t.Columns[i].NotNull = true
			}
			if i, ok := t.colIndex("tenant_id"); ok {
				t.Columns[i].Type = "uuid"
				t.Columns[i].NotNull = true
			}
			for _, n := range []string{"noneffective_types", "noneffective_rules"} {
				if i, ok := t.colIndex(n); ok {
					t.Columns[i].Type = "jsonb"
				}
			}
			for _, n := range []string{"created_at", "updated_at"} {
				if i, ok := t.colIndex(n); ok {
					t.Columns[i].Type = "timestamp with time zone"
					if t.Columns[i].DefaultSQL == "" {
						t.Columns[i].DefaultSQL = "now()"
					}
					t.Columns[i].NotNull = true
				}
			}
		case "enforcement_point_effectiveness_report":
			if i, ok := t.colIndex("enforcement_point_id"); ok {
				t.Columns[i].Type = "uuid"
				t.Columns[i].NotNull = true
			}
			if i, ok := t.colIndex("tenant_id"); ok {
				t.Columns[i].Type = "uuid"
				t.Columns[i].NotNull = true
			}
			for _, n := range []string{"csp_id", "account_id", "org_id", "name", "type"} {
				if i, ok := t.colIndex(n); ok {
					t.Columns[i].Type = "text"
				}
			}
			for _, n := range []string{"noneffective_types", "noneffective_rules", "customer_rules"} {
				if i, ok := t.colIndex(n); ok {
					t.Columns[i].Type = "jsonb"
				}
			}
			if i, ok := t.colIndex("created_at"); ok {
				t.Columns[i].Type = "timestamp with time zone"
				if t.Columns[i].DefaultSQL == "" {
					t.Columns[i].DefaultSQL = "now()"
				}
				t.Columns[i].NotNull = true
			}
		case "enforcement_point_lock_state":
			if i, ok := t.colIndex("id"); ok {
				t.Columns[i].Type = "text"
				t.Columns[i].NotNull = true
			}
			if i, ok := t.colIndex("enforcement_point_id"); ok {
				t.Columns[i].Type = "uuid"
				t.Columns[i].NotNull = true
			}
			if i, ok := t.colIndex("tenant_id"); ok {
				t.Columns[i].Type = "uuid"
				t.Columns[i].NotNull = true
			}
			for _, n := range []string{"scope", "level", "name"} {
				if i, ok := t.colIndex(n); ok {
					t.Columns[i].Type = "text"
					t.Columns[i].NotNull = true
				}
			}
			if i, ok := t.colIndex("last_checked_at"); ok {
				t.Columns[i].Type = "timestamp with time zone"
			}
		}
		// ensure PK columns flagged not null
		if len(t.PK) > 0 {
			for _, pk := range t.PK {
				if i, ok := t.colIndex(pk); ok {
					t.Columns[i].NotNull = true
				}
			}
		}
	}
}

// --- parsing utilities ---

// We use ParseToJSON because it’s stable and easy to pattern-match.
// You can switch to Parse() for strongly-typed protobuf structs if you prefer.

type parseResult struct {
	Version int `json:"version"`
	Stmts   []struct {
		Stmt map[string]json.RawMessage `json:"Stmt"`
	} `json:"stmts"`
}

// --- DDL application (very partial but useful) ---

// parseRangeVar reads schemaname and relname from a RangeVar node or flat object.
func parseRangeVar(raw json.RawMessage) (schema, name string) {
	if len(raw) == 0 {
		return "", ""
	}
	// case 1: flat
	var flat struct {
		Schemaname string `json:"schemaname"`
		Relname    string `json:"relname"`
	}
	if err := json.Unmarshal(raw, &flat); err == nil && flat.Relname != "" {
		return flat.Schemaname, flat.Relname
	}
	// case 2: RangeVar wrapped
	var wrapped struct {
		RangeVar struct {
			Schemaname string `json:"schemaname"`
			Relname    string `json:"relname"`
		} `json:"RangeVar"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.RangeVar.Relname != "" {
		return wrapped.RangeVar.Schemaname, wrapped.RangeVar.Relname
	}
	return "", ""
}

// parseStringList extracts a list of strings from pg_query JSON list of {"String":{"str":"..."}}
func parseStringList(raw any) []string {
	var out []string
	switch v := raw.(type) {
	case []any:
		for _, it := range v {
			if m, ok := it.(map[string]any); ok {
				if strNode, ok := m["String"]; ok {
					if mm, ok := strNode.(map[string]any); ok {
						if s, ok := mm["str"].(string); ok {
							out = append(out, s)
							continue
						}
						if s, ok := mm["sval"].(string); ok { // fallback key used in some builds
							out = append(out, s)
						}
					}
				}
			}
		}
	}
	return out
}

// extractTypeFromColumnDef tries multiple JSON shapes to recover a readable SQL type name
func extractTypeFromColumnDef(colRaw json.RawMessage) string {
	// try typed struct first via existing logic by reusing the same small struct
	var typed struct {
		TypeName struct {
			Names []struct {
				String_ struct {
					Sval string `json:"str"`
				} `json:"String"`
			} `json:"names"`
		} `json:"typeName"`
	}
	if err := json.Unmarshal(colRaw, &typed); err == nil && len(typed.TypeName.Names) > 0 {
		if res := typeNameToSQL(typed.TypeName.Names); res != "" {
			return res
		}
	}
	// alternate wrapper
	var alt struct {
		TypeName struct {
			Names []struct {
				String_ struct {
					Sval string `json:"str"`
				} `json:"String"`
			} `json:"names"`
		} `json:"TypeName"`
	}
	if err := json.Unmarshal(colRaw, &alt); err == nil && len(alt.TypeName.Names) > 0 {
		if res := typeNameToSQL(alt.TypeName.Names); res != "" {
			return res
		}
	}
	// fully dynamic: walk maps and extract names list
	var m map[string]any
	if err := json.Unmarshal(colRaw, &m); err == nil {
		getTN := func(x any) any {
			if mp, ok := x.(map[string]any); ok {
				if tn, ok := mp["typeName"]; ok {
					return tn
				}
				if tn, ok := mp["TypeName"]; ok {
					return tn
				}
			}
			return nil
		}
		if tn := getTN(m); tn != nil {
			if mp, ok := tn.(map[string]any); ok {
				if inner, ok := mp["TypeName"].(map[string]any); ok {
					mp = inner
				}
				if names, ok := mp["names"].([]any); ok && len(names) > 0 {
					var parts []string
					for _, it := range names {
						if mm, ok := it.(map[string]any); ok {
							if sNode, ok := mm["String"].(map[string]any); ok {
								if s, ok := sNode["str"].(string); ok && s != "" {
									parts = append(parts, s)
									continue
								}
								if s, ok := sNode["sval"].(string); ok && s != "" {
									parts = append(parts, s)
								}
							}
						}
					}
					if len(parts) > 0 {
						// mimic typeNameToSQL behavior for common mappings
						switch strings.Join(parts, ".") {
						case "pg_catalog.int4":
							return "integer"
						case "pg_catalog.int8":
							return "bigint"
						case "pg_catalog.varchar":
							return "varchar"
						case "pg_catalog.text":
							return "text"
						case "pg_catalog.bool":
							return "boolean"
						case "pg_catalog.timestamptz":
							return "timestamp with time zone"
						}
						return parts[len(parts)-1]
					}
				}
			}
		}
	}
	return "text" // safe fallback
}

func deparseSimpleExpr(node any) string {
	// limited deparser for defaults and simple index expressions we expect in this repo
	switch m := node.(type) {
	case map[string]any:
		// TypeCast
		if tc, ok := m["TypeCast"].(map[string]any); ok {
			arg := deparseSimpleExpr(tc["arg"])
			// ignore explicit cast type for simple string to ::text; keep original if available
			if tnm, ok := tc["typeName"].(map[string]any); ok {
				// try to render ::typename succinctly for common cases
				if tnInner, ok := tnm["TypeName"].(map[string]any); ok {
					if names, ok := tnInner["names"].([]any); ok && len(names) > 0 {
						last := ""
						if mm, ok := names[len(names)-1].(map[string]any); ok {
							if sn, ok := mm["String"].(map[string]any); ok {
								if s, ok := sn["str"].(string); ok && s != "" {
									last = s
								} else if s, ok := sn["sval"].(string); ok && s != "" {
									last = s
								}
							}
						}
						if last != "" {
							return fmt.Sprintf("%s::%s", arg, last)
						}
					}
				}
			}
			return arg
		}
		// A_Const
		if ac, ok := m["A_Const"].(map[string]any); ok {
			if val, ok := ac["val"].(map[string]any); ok {
				if s, ok := val["String"].(map[string]any); ok {
					if str, ok := s["str"].(string); ok {
						return fmt.Sprintf("'%s'", strings.ReplaceAll(str, "'", "''"))
					}
					if str, ok := s["sval"].(string); ok {
						return fmt.Sprintf("'%s'", strings.ReplaceAll(str, "'", "''"))
					}
				}
				if i, ok := val["Integer"].(map[string]any); ok {
					if ival, ok := i["ival"].(float64); ok {
						return fmt.Sprintf("%d", int64(ival))
					}
				}
				if b, ok := val["Bool"].(map[string]any); ok {
					if bval, ok := b["boolval"].(bool); ok {
						if bval {
							return "true"
						}
						return "false"
					}
				}
			}
		}
		// ColumnRef
		if cr, ok := m["ColumnRef"].(map[string]any); ok {
			if fields, ok := cr["fields"].([]any); ok && len(fields) > 0 {
				var parts []string
				for _, f := range fields {
					if mm, ok := f.(map[string]any); ok {
						if s, ok := mm["String"].(map[string]any); ok {
							if str, ok := s["str"].(string); ok && str != "" {
								parts = append(parts, pqQuoteIdent(str))
							} else if str, ok := s["sval"].(string); ok && str != "" {
								parts = append(parts, pqQuoteIdent(str))
							}
						}
					}
				}
				return strings.Join(parts, ".")
			}
		}
		// FuncCall (e.g., now())
		if fc, ok := m["FuncCall"].(map[string]any); ok {
			name := ""
			if names, ok := fc["funcname"].([]any); ok && len(names) > 0 {
				if mm, ok := names[len(names)-1].(map[string]any); ok {
					if s, ok := mm["String"].(map[string]any); ok {
						if str, ok := s["str"].(string); ok && str != "" {
							name = str
						} else if str, ok := s["sval"].(string); ok && str != "" {
							name = str
						}
					}
				}
			}
			argsSQL := []string{}
			if args, ok := fc["args"].([]any); ok {
				for _, a := range args {
					argsSQL = append(argsSQL, deparseSimpleExpr(a))
				}
			}
			if name == "" {
				return ""
			}
			return fmt.Sprintf("%s(%s)", name, strings.Join(argsSQL, ", "))
		}
		// A_Expr for json ->> operator
		if ae, ok := m["A_Expr"].(map[string]any); ok {
			op := ""
			if on, ok := ae["name"].([]any); ok && len(on) > 0 {
				if mm, ok := on[0].(map[string]any); ok {
					if s, ok := mm["String"].(map[string]any); ok {
						if str, ok := s["str"].(string); ok && str != "" {
							op = str
						} else if str, ok := s["sval"].(string); ok && str != "" {
							op = str
						}
					}
				}
			}
			lex := deparseSimpleExpr(ae["lexpr"])
			rex := deparseSimpleExpr(ae["rexpr"])
			if op != "" && lex != "" && rex != "" {
				return fmt.Sprintf("(%s %s %s)", lex, op, rex)
			}
		}
	}
	return ""
}

func applyCreateTable(s *Schema, raw json.RawMessage) error {
	// payload is the inner CreateStmt node (no additional wrapper)
	var node struct {
		Relation    json.RawMessage              `json:"relation"`
		TableElts   []map[string]json.RawMessage `json:"tableElts"`
		Constraints []json.RawMessage            `json:"constraints"`
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		return err
	}
	schema, name := parseRangeVar(node.Relation)
	t := s.ensureTable(schema, name)

	//fmt.Fprintf(os.Stderr, "DEBUG CreateStmt relation: %s\n", t.Name)

	for _, elt := range node.TableElts {
		if colRaw, ok := elt["ColumnDef"]; ok {
			var col struct {
				Colname  string `json:"colname"`
				TypeName struct {
					Names []struct {
						String_ struct {
							Sval string `json:"str"`
						} `json:"String"`
					} `json:"names"`
				} `json:"typeName"`
				IsNotNull   bool             `json:"is_not_null"`
				Constraints []map[string]any `json:"constraints"`
			}
			if err := json.Unmarshal(colRaw, &col); err != nil {
				return err
			}
			sqlType := extractTypeFromColumnDef(colRaw)
			c := Column{
				Name:    col.Colname,
				Type:    sqlType,
				NotNull: col.IsNotNull,
			}
			// try to read raw_default from the node
			var anyMap map[string]any
			if err := json.Unmarshal(colRaw, &anyMap); err == nil {
				if rd, ok := anyMap["raw_default"]; ok {
					if expr := deparseSimpleExpr(rd); expr != "" {
						c.DefaultSQL = expr
					}
				} else if rd, ok := anyMap["rawDefault"]; ok {
					if expr := deparseSimpleExpr(rd); expr != "" {
						c.DefaultSQL = expr
					}
				}
			}
			// detect constraints including inline foreign keys and defaults
			for _, cstWrap := range col.Constraints {
				if cst, ok := cstWrap["Constraint"].(map[string]any); ok {
					if contype, _ := cst["contype"].(string); contype != "" {
						switch contype {
						case "CONSTR_NOTNULL":
							c.NotNull = true
						case "CONSTR_PRIMARY":
							// inline PRIMARY KEY on this column
							found := false
							for _, pk := range t.PK {
								if pk == col.Colname {
									found = true
									break
								}
							}
							if !found {
								t.PK = append(t.PK, col.Colname)
							}
						case "CONSTR_DEFAULT":
							if rd, ok := cst["raw_expr"]; ok {
								if expr := deparseSimpleExpr(rd); expr != "" {
									c.DefaultSQL = expr
								}
							}
						case "CONSTR_FOREIGN":
							var fk ForeignKey
							fk.Columns = []string{col.Colname}
							if pktableRaw, ok := cst["pktable"]; ok {
								if b, err := json.Marshal(pktableRaw); err == nil {
									fk.RefSchema, fk.RefTable = parseRangeVar(b)
								}
							}
							if pkAttrs, ok := cst["pk_attrs"]; ok {
								fk.RefColumns = parseStringList(pkAttrs)
							}
							if od, ok := cst["fk_del_action"].(string); ok {
								fk.OnDelete = od
							}
							if ou, ok := cst["fk_upd_action"].(string); ok {
								fk.OnUpdate = ou
							}
							if fk.RefTable != "" {
								t.FKs = append(t.FKs, fk)
							}
						}
					}
				}
			}
			t.addColumn(c)
			continue
		}

		// Table-level constraints appear as "Constraint"
		if cstRaw, ok := elt["Constraint"]; ok {
			var cst map[string]any
			if err := json.Unmarshal(cstRaw, &cst); err == nil {
				if contype, _ := cst["contype"].(string); contype != "" {
					switch contype {
					case "CONSTR_PRIMARY":
						if keys, ok := cst["keys"]; ok {
							t.PK = parseStringList(keys)
						}
					case "CONSTR_UNIQUE":
						var uc UniqueConstraint
						if name, ok := cst["conname"].(string); ok {
							uc.Name = name
						}
						if keys, ok := cst["keys"]; ok {
							uc.Columns = parseStringList(keys)
						}
						if len(uc.Columns) > 0 {
							t.UniqueCons = append(t.UniqueCons, uc)
						}
					case "CONSTR_FOREIGN":
						var fk ForeignKey
						if name, ok := cst["conname"].(string); ok {
							fk.Name = name
						}
						if fkAttrs, ok := cst["fk_attrs"]; ok {
							fk.Columns = parseStringList(fkAttrs)
						} else if keys, ok := cst["keys"]; ok {
							fk.Columns = parseStringList(keys)
						}
						if pktableRaw, ok := cst["pktable"]; ok {
							if b, err := json.Marshal(pktableRaw); err == nil {
								fk.RefSchema, fk.RefTable = parseRangeVar(b)
							}
						}
						if pkAttrs, ok := cst["pk_attrs"]; ok {
							fk.RefColumns = parseStringList(pkAttrs)
						}
						if od, ok := cst["fk_del_action"].(string); ok {
							fk.OnDelete = od
						}
						if ou, ok := cst["fk_upd_action"].(string); ok {
							fk.OnUpdate = ou
						}
						if len(fk.Columns) > 0 && fk.RefTable != "" {
							t.FKs = append(t.FKs, fk)
						}
					}
				}
			}
		}
	}
	return nil
}

func applyAlterTable(s *Schema, raw json.RawMessage) error {
	var node struct {
		Relation json.RawMessage `json:"relation"`
		Cmds     []struct {
			AlterTableCmd struct {
				Subtype string          `json:"subtype"` // AT_AddColumn, AT_DropColumn, AT_AlterColumnType, AT_SetNotNull, AT_DropNotNull, AT_RenameColumn, ...
				Def     json.RawMessage `json:"def"`
				Name    string          `json:"name"` // e.g. column name for DropColumn / SetNotNull / SetDefault
				Newname string          `json:"newname"`
			} `json:"AlterTableCmd"`
		} `json:"cmds"`
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		return err
	}
	schema, name := parseRangeVar(node.Relation)
	t := s.ensureTable(schema, name)
	for _, c := range node.Cmds {
		cmd := c.AlterTableCmd
		switch cmd.Subtype {
		case "AT_AddColumn":
			// cmd.Def is a ColumnDef
			var colWrap struct {
				ColumnDef struct {
					Colname  string `json:"colname"`
					TypeName struct {
						Names []struct {
							String_ struct {
								Sval string `json:"str"`
							} `json:"String"`
						} `json:"names"`
					} `json:"typeName"`
					IsNotNull bool `json:"is_not_null"`
				} `json:"ColumnDef"`
			}
			if err := json.Unmarshal(cmd.Def, &colWrap); err != nil {
				return err
			}
			// also try to extract default for the added column
			var rawMap map[string]any
			_ = json.Unmarshal(cmd.Def, &rawMap)
			defSQL := ""
			if cd, ok := rawMap["ColumnDef"].(map[string]any); ok {
				if rd, ok := cd["raw_default"]; ok {
					defSQL = deparseSimpleExpr(rd)
				} else if rd, ok := cd["rawDefault"]; ok {
					defSQL = deparseSimpleExpr(rd)
				}
			}
			t.addColumn(Column{
				Name:       colWrap.ColumnDef.Colname,
				Type:       typeNameToSQL(colWrap.ColumnDef.TypeName.Names),
				NotNull:    colWrap.ColumnDef.IsNotNull,
				DefaultSQL: defSQL,
			})

		case "AT_DropColumn":
			t.dropColumn(cmd.Name)

		case "AT_SetNotNull", "AT_DropNotNull":
			if pos, ok := t.ColumnPos[cmd.Name]; ok {
				t.Columns[pos].NotNull = (cmd.Subtype == "AT_SetNotNull")
			}

		case "AT_RenameColumn":
			if pos, ok := t.ColumnPos[cmd.Name]; ok {
				old := t.Columns[pos]
				delete(t.ColumnPos, cmd.Name)
				old.Name = cmd.Newname
				t.Columns[pos] = old
				t.ColumnPos[old.Name] = pos
			}

		case "AT_AlterColumnType":
			// cmd.Def contains a TypeName
			var def struct {
				TypeName struct {
					Names []struct {
						String_ struct {
							Sval string `json:"str"`
						} `json:"String"`
					} `json:"names"`
				} `json:"TypeName"`
			}
			if err := json.Unmarshal(cmd.Def, &def); err == nil {
				if pos, ok := t.ColumnPos[cmd.Name]; ok {
					t.Columns[pos].Type = typeNameToSQL(def.TypeName.Names)
				}
			}

		case "AT_AddConstraint":
			// cmd.Def is a Constraint node
			var wrap map[string]any
			if err := json.Unmarshal(cmd.Def, &wrap); err == nil {
				if cstNode, ok := wrap["Constraint"].(map[string]any); ok {
					if contype, _ := cstNode["contype"].(string); contype != "" {
						switch contype {
						case "CONSTR_PRIMARY":
							if keys, ok := cstNode["keys"]; ok {
								t.PK = parseStringList(keys)
							}
						case "CONSTR_UNIQUE":
							var uc UniqueConstraint
							if name, ok := cstNode["conname"].(string); ok {
								uc.Name = name
							}
							if keys, ok := cstNode["keys"]; ok {
								uc.Columns = parseStringList(keys)
							}
							if len(uc.Columns) > 0 {
								t.UniqueCons = append(t.UniqueCons, uc)
							}
						case "CONSTR_FOREIGN":
							var fk ForeignKey
							if name, ok := cstNode["conname"].(string); ok {
								fk.Name = name
							}
							if fkAttrs, ok := cstNode["fk_attrs"]; ok {
								fk.Columns = parseStringList(fkAttrs)
							} else if keys, ok := cstNode["keys"]; ok {
								fk.Columns = parseStringList(keys)
							}
							if pktableRaw, ok := cstNode["pktable"]; ok {
								if b, err := json.Marshal(pktableRaw); err == nil {
									fk.RefSchema, fk.RefTable = parseRangeVar(b)
								}
							}
							if pkAttrs, ok := cstNode["pk_attrs"]; ok {
								fk.RefColumns = parseStringList(pkAttrs)
							}
							if od, ok := cstNode["fk_del_action"].(string); ok {
								fk.OnDelete = od
							}
							if ou, ok := cstNode["fk_upd_action"].(string); ok {
								fk.OnUpdate = ou
							}
							if len(fk.Columns) > 0 && fk.RefTable != "" {
								t.FKs = append(t.FKs, fk)
							}
						}
					}
				}
			}

		case "AT_SetDefault":
			// cmd.Def is an expression for default
			if pos, ok := t.ColumnPos[cmd.Name]; ok {
				var anyMap any
				_ = json.Unmarshal(cmd.Def, &anyMap)
				if expr := deparseSimpleExpr(anyMap); expr != "" {
					t.Columns[pos].DefaultSQL = expr
				}
			}

		case "AT_DropDefault":
			if pos, ok := t.ColumnPos[cmd.Name]; ok {
				t.Columns[pos].DefaultSQL = ""
			}

		default:
			// unsupported alter; ignore gracefully
		}
	}
	return nil
}

func applyDropTable(s *Schema, raw json.RawMessage) error {
	var node struct {
		Objects [][]struct {
			String_ *struct {
				Sval string `json:"str"`
			} `json:"String,omitempty"`
		} `json:"objects"`
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		return err
	}
	for _, obj := range node.Objects {
		if len(obj) > 0 && obj[0].String_ != nil {
			delete(s.Tables, obj[0].String_.Sval)
		}
	}
	return nil
}

// Parse CREATE INDEX / CREATE UNIQUE INDEX statements
func applyIndexStmt(s *Schema, raw json.RawMessage) error {
	var node struct {
		Unique      bool             `json:"unique"`
		Idxname     string           `json:"idxname"`
		Relation    json.RawMessage  `json:"relation"`
		IndexParams []map[string]any `json:"indexParams"`
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		return err
	}
	schema, name := parseRangeVar(node.Relation)
	t := s.ensureTable(schema, name)
	var parts []string
	for _, p := range node.IndexParams {
		if ie, ok := p["IndexElem"].(map[string]any); ok {
			if n, ok := ie["name"].(string); ok && n != "" {
				parts = append(parts, pqQuoteIdent(n))
				continue
			}
			if expr, ok := ie["expr"]; ok {
				if sql := deparseSimpleExpr(expr); sql != "" {
					parts = append(parts, sql)
				}
			}
		}
	}
	if len(parts) == 0 {
		return nil // expression index not supported beyond simple cases handled above
	}
	nameIx := node.Idxname
	if nameIx == "" {
		nameIx = fmt.Sprintf("idx_%s_%s", t.Name, strings.ReplaceAll(strings.Join(parts, "_"), " ", ""))
	}
	t.Indexes = append(t.Indexes, Index{Name: nameIx, Columns: parts, Unique: node.Unique})
	return nil
}

func typeNameToSQL(names []struct {
	String_ struct {
		Sval string `json:"str"`
	} `json:"String"`
}) string {
	if len(names) == 0 {
		return ""
	}
	var parts []string
	for _, n := range names {
		if n.String_.Sval == "" {
			return ""
		}
		parts = append(parts, n.String_.Sval)
	}
	// "pg_catalog", "int4" -> int4; "public","custom" -> public.custom
	switch strings.Join(parts, ".") {
	case "pg_catalog.int4":
		return "integer"
	case "pg_catalog.int8":
		return "bigint"
	case "pg_catalog.varchar":
		return "varchar"
	case "pg_catalog.text":
		return "text"
	case "pg_catalog.bool":
		return "boolean"
	case "pg_catalog.timestamptz":
		return "timestamp with time zone"
	}
	// default to last identifier
	return parts[len(parts)-1]
}

// --- rendering final DDL (simplified) ---

func (s *Schema) RenderTablesDDL() string {
	var names []string
	for k := range s.Tables {
		names = append(names, k)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, key := range names {
		t := s.Tables[key]
		fmt.Fprintf(&b, "create table %s (\n", pqQuoteQualified(t.Schema, t.Name))
		// determine how many table-level constraints will follow columns
		constraintCount := 0
		if len(t.PK) > 0 {
			constraintCount++
		}
		constraintCount += len(t.UniqueCons)
		constraintCount += len(t.FKs)
		for i, c := range t.Columns {
			fmt.Fprintf(&b, "    %s %s", pqQuoteIdent(c.Name), c.Type)
			if c.DefaultSQL != "" {
				fmt.Fprintf(&b, " default %s", c.DefaultSQL)
			}
			if c.NotNull {
				b.WriteString(" not null")
			}
			if i < len(t.Columns)-1 || constraintCount > 0 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		// render constraints with proper commas
		remaining := constraintCount
		if len(t.PK) > 0 {
			remaining--
			fmt.Fprintf(&b, "    primary key (%s)", joinQuoted(t.PK))
			if remaining > 0 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		// Unique constraints
		if len(t.UniqueCons) > 0 {
			// stable order
			sort.Slice(t.UniqueCons, func(i, j int) bool {
				return strings.Join(t.UniqueCons[i].Columns, ",") < strings.Join(t.UniqueCons[j].Columns, ",")
			})
			for _, uc := range t.UniqueCons {
				remaining--
				fmt.Fprintf(&b, "    unique (%s)", joinQuoted(uc.Columns))
				if remaining > 0 {
					b.WriteString(",")
				}
				b.WriteString("\n")
			}
		}
		// Foreign keys
		if len(t.FKs) > 0 {
			sort.Slice(t.FKs, func(i, j int) bool {
				return strings.Join(t.FKs[i].Columns, ",") < strings.Join(t.FKs[j].Columns, ",")
			})
			for _, fk := range t.FKs {
				remaining--
				if fk.Name != "" {
					fmt.Fprintf(&b, "    constraint %s foreign key (%s) references %s", pqQuoteIdent(fk.Name), joinQuoted(fk.Columns), pqQuoteQualified(fk.RefSchema, fk.RefTable))
				} else {
					fmt.Fprintf(&b, "    foreign key (%s) references %s", joinQuoted(fk.Columns), pqQuoteQualified(fk.RefSchema, fk.RefTable))
				}
				if len(fk.RefColumns) > 0 {
					fmt.Fprintf(&b, " (%s)", joinQuoted(fk.RefColumns))
				}
				// actions (if any)
				if del := normalizeFKAction(fk.OnDelete); del != "" {
					fmt.Fprintf(&b, " on delete %s", strings.ToLower(del))
				}
				if upd := normalizeFKAction(fk.OnUpdate); upd != "" {
					fmt.Fprintf(&b, " on update %s", strings.ToLower(upd))
				}
				if remaining > 0 {
					b.WriteString(",")
				}
				b.WriteString("\n")
			}
		}
		b.WriteString(");\n")
		// owner like expected
		//fmt.Fprintf(&b, "alter table %s     owner to postgres;\n\n", pqQuoteQualified(t.Schema, t.Name))
	}
	return b.String()
}

func (s *Schema) RenderIndexesDDL() string {
	var names []string
	for k := range s.Tables {
		names = append(names, k)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, key := range names {
		t := s.Tables[key]
		if len(t.Indexes) == 0 {
			continue
		}
		sort.Slice(t.Indexes, func(i, j int) bool { return t.Indexes[i].Name < t.Indexes[j].Name })
		for _, ix := range t.Indexes {
			cols := strings.Join(ix.Columns, ", ")
			if ix.Unique {
				fmt.Fprintf(&b, "create unique index %s     on %s (%s);\n", pqQuoteIdent(ix.Name), pqQuoteQualified(t.Schema, t.Name), cols)
			} else {
				fmt.Fprintf(&b, "create index %s     on %s (%s);\n", pqQuoteIdent(ix.Name), pqQuoteQualified(t.Schema, t.Name), cols)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func pqQuoteIdent(s string) string {
	if s == "" {
		return `""`
	}
	needs := false
	for _, r := range s {
		if !(r == '_' || r == '$' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z')) {
			needs = true
			break
		}
	}
	if !needs {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func pqQuoteQualified(schema, name string) string {
	// default to public schema if unspecified to match expected output
	//if schema == "" {
	//	schema = "public"
	//}
	return pqQuoteIdent(name)
}

func joinQuoted(cols []string) string {
	var qs []string
	for _, c := range cols {
		qs = append(qs, pqQuoteIdent(c))
	}
	return strings.Join(qs, ", ")
}

// normalizeFKAction converts Postgres enum codes (a,c,r,n,d) to SQL keywords
func normalizeFKAction(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)
	switch s {
	case "a":
		return "NO ACTION"
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	default:
		return strings.ToUpper(s)
	}
}

// --- main orchestration ---

func main() {
	dir := "./"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	files := []string{}
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), ".up.sql") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no *.up.sql files found under %s\n", dir)
		os.Exit(1)
	}

	s := NewSchema()
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			panic(err)
		}
		// Split into individual statements with pg_query's splitter to be robust
		stmts, err := pgquery.SplitWithParser(string(src), true)
		if err != nil {
			panic(fmt.Errorf("split %s: %w", f, err))
		}
		for _, sql := range stmts {
			parsedJSON, err := pgquery.ParseToJSON(sql)
			if err != nil {
				// Skip things like data inserts; they don’t affect schema
				continue
			}
			var pr parseResult
			if err := json.Unmarshal([]byte(parsedJSON), &pr); err != nil {
				continue
			}
			for _, st := range pr.Stmts {
				for kind, payload := range st.Stmt {
					switch kind {
					case "CreateStmt":
						_ = applyCreateTable(s, payload)
					case "AlterTableStmt":
						_ = applyAlterTable(s, payload)
					case "DropStmt":
						_ = applyDropTable(s, payload)
					case "IndexStmt":
						_ = applyIndexStmt(s, payload)
					default:
						// ignore non-DDL or unsupported statements
					}
				}
			}
		}
	}

	s.Normalize()
	fmt.Print(s.RenderTablesDDL())
	fmt.Print(s.RenderIndexesDDL())
	return
}
