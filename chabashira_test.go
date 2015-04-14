package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"testing"
)

func TestParseStruct(t *testing.T) {
	var tables []table

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", `
package sample

// Not target
type Dummy struct {
	foo int
}

// db:"entity"
type Piyo struct {
	Id			int64  `+"`"+`db:"pk"`+"`"+`
	SomeValue	string
}

// db:"entity"
type Fragments struct {
	HiddenPk int64  `+"`"+`db:"pk"`+"`"+`
	Id       int64  `+"`"+`db:"unique"`+"`"+`
	Version  uint16  `+"`"+`db:"unique" default:"0"`+"`"+`
	Size     int32
	Addr     string `+"`"+`db:"unique"`+"`"+`
	PiyoId	 int64  `+"`"+`refer:""`+"`"+`
}

`, parser.ParseComments)

	if err != nil {
		t.Error(err)
	}
	tables = parseFile(fset, file, tables)

	if len(tables) != 2 {
		t.Error("# of tables ", len(tables), " should be 2")
	}
	fit := false
	for _, tbl := range(tables) {
		if tbl.Name != "Fragments" {
			continue
		}
		fit = true
		if tbl.Pk != "HiddenPk" {
			t.Error("Fail to get primary key")
		}
		if len(tbl.Columns) != 6 {
			t.Error("Fail to get column ", tbl.Columns)
		}
		if len(tbl.Index) != 3 {
			t.Error("Fail to get index", tbl.Index)
		}

		buf := bytes.NewBufferString("")
		ts := []table{tbl}
		putMigrate(ts, buf)
		if buf.String() !=
		`create_table 'fragments', primary_key:'hidden_pk' do |t|
  t.integer :id, null:false, limit:8
  t.integer :version, null:false, default:0, limit:2
  t.integer :size, null:false, limit:4
  t.string :addr, null:false
  t.references :piyo, limit:8
end
add_index :fragments, [:id, :version, :addr], unique:true

` {
			t.Error("Fail to put in AR format : '", buf.String(), "'")
		}

		buf.Truncate(0)
		putNames(ts, "main", buf)
		if buf.String() != `package main
//  Fragments
func (* Fragments )  hidden_pk () string {
	return "hidden_pk"
}
func (* Fragments )  id () string {
	return "id"
}
func (* Fragments )  version () string {
	return "version"
}
func (* Fragments )  size () string {
	return "size"
}
func (* Fragments )  addr () string {
	return "addr"
}
func (* Fragments )  piyo_id () string {
	return "piyo_id"
}
` {
			t.Error("Fail to put in name file : '", buf.String(), "'")
		}
	}
	if !fit {
		t.Error("Fail to get table name")
	}
}
