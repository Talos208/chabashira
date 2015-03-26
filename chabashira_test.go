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

// db:"entity"
type Fragments struct {
	HiddenPk int64  `+"`"+`db:"pk"`+"`"+`
	Id       int64  `+"`"+`db:"unique"`+"`"+`
	Version  int64  `+"`"+`db:"unique" default:"0"`+"`"+`
	Addr     string `+"`"+`db:"unique"`+"`"+`
}    `, parser.ParseComments)

	if err != nil {
		t.Error(err)
	}
	tables = parseFile(fset, file, tables)

	if len(tables) != 1 {
		t.Error("# of tables ", len(tables), " should be 1")
	}
	tbl := tables[0]
	if tbl.Pk != "HiddenPk" {
		t.Error("Fail to get primary key")
	}
	if tbl.Name != "Fragments" {
		t.Error("Fail to get table name")
	}
	if len(tbl.Columns) != 4 {
		t.Error("Fail to get column")
	}
	if len(tbl.Index) != 3 {
		t.Error("Fail to get index")
	}

	buf := bytes.NewBufferString("")
	putMigrate(tables, buf)
	if buf.String() !=
		`create_table 'fragments', primary_key:'hidden_pk' do |t|
  t.integer :id, null:false
  t.integer :version, null:false
  t.string :addr, null:false
end
add_index :fragments, [:id, :version, :addr], unique:true

` {
		t.Error("Fail to put in AR format : '", buf.String(), "'")
	}
}
