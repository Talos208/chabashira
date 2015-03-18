package main

import (
	"fmt"
	"github.com/naoina/go-stringutil"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"reflect"
	"strings"
    "flag"
    "path/filepath"
    "os"
)

type column struct {
	Type string
	Name string
	Opt  map[string]string
}

type table struct {
	Name    string
	Columns []column
	Pk      string
	Index   []string
}

// Read schema info. from struct
func parseStruct(gd *ast.GenDecl, st *ast.StructType) table {
	tbl := table{"", make([]column, 0), "", make([]string, 0)}
	for _, s := range gd.Specs {
		if typ, ok := s.(*ast.TypeSpec); ok {
			tbl.Name = typ.Name.Name
			break
		}
	}
EachField:
	for _, fld := range st.Fields.List {
		col := column{"", "", make(map[string]string)}
		tp := fld.Type
		var tn string
		if id, ok := tp.(*ast.Ident); ok {
			tn = id.Name
		}

		if id, ok := tp.(*ast.SelectorExpr); ok {
			tn = id.Sel.Name
		}

		if ar, ok := tp.(*ast.ArrayType); ok {
			tn = ar.Elt.(*ast.Ident).Name + "[]"
		}

		// Convert Go's type to SQL type
		switch tn {
		case "int", "int16", "int32", "int64", "uint", "uint16", "uint32", "uint64":
			col.Type = "integer"
		case "string":
			col.Type = "string"
		case "byte[]":
			col.Type = "binary"
		case "Time":
			col.Type = "timestamp"
		default:
			log.Print(reflect.TypeOf(tp).String(), tp)
		}

		col.Name = fld.Names[0].String() // is it preferd to use stringutil.join() ?

		// タグの解釈
		if fld.Tag != nil {
			tgs := strings.Split(strings.Trim(fld.Tag.Value, "`"), " ")
			for _, t := range tgs {
				if len(t) == 0 {
					continue
				}
				kv := strings.Split(t, ":")
				kv[1] = strings.Trim(kv[1], "\"")

				switch kv[0] {
				case "db":
					switch kv[1] {
					case "-":   // no target.
						continue EachField
					case "pk":  // primary key
						tbl.Pk = col.Name
					case "unique":  // unique constraint
						tbl.Index = append(tbl.Index, col.Name)
					}
				case "size":    // size constraint
					col.Opt["limit"] = kv[1]
				case "default": // default value
					col.Opt["default"] = kv[1]
				case "column":  // column name
					col.Name = kv[1]
				case "refer":   // reference to another table
					col.Type = "references"
					if len(kv[1]) > 0 {
						col.Name = kv[1]
					} else {
						col.Name = strings.TrimSuffix(col.Name, "Id")
					}
				}
			}
		}

		tbl.Columns = append(tbl.Columns, col)
	}

	return tbl
}

// Search target struct from file
func parseFile(fset *token.FileSet, file *ast.File, tables []table) []table {
	if tables == nil {
		tables = make([]table, 0)
	}
	cm := ast.NewCommentMap(fset, file, file.Comments)

	for node, cgs := range cm {
//      To process module comment, here is a place for.
//		if _, ok := node.(*ast.File); ok {
//			for _, cg := range cgs {
//				for _, c := range cg.List {
//					log.Println(c.Text)
//				}
//			}
//		}
        if gd, ok := node.(*ast.GenDecl); ok {
			for _, spec := range gd.Specs {
				if ts, ok := spec.(*ast.TypeSpec); !ok {
                    continue
                } else if st, ok2 := ts.Type.(*ast.StructType); !ok2 {
                    continue
                } else {
                Outer:
                    for _, cg := range cgs {
                        for _, c := range cg.List {
                            if strings.Contains(c.Text, `db:"entity"`) {
                                tables = append(tables, parseStruct(gd, st))
                                break Outer
                            }
                        }
                    }
				}
			}
		}
	}

	return tables
}

// Output in Rails's migration file format.
func putMigrate(tables []table) {
	for _, tbl := range tables {
		ixFlg := true
		if len(tbl.Index) == 1 {
			ixFlg = false
		}

		fmt.Printf("create_table '%s'", stringutil.ToSnakeCase(tbl.Name))
		if len(tbl.Pk) == 0 {
			fmt.Print(", id:false")
		}
		fmt.Printf(" do |t|\n")
		for _, col := range tbl.Columns {
			if col.Name == tbl.Pk {
				continue
			}
			cs := fmt.Sprintf("t.%s :%s", col.Type, stringutil.ToSnakeCase(col.Name))
			if !ixFlg && col.Name == tbl.Index[0] {
				cs = cs + ", unique:true"
			}
			fmt.Println(" ", cs)
		}
		fmt.Println("end")
		if len(tbl.Index) > 1 {
			is := fmt.Sprintf("add_index :%s, [", stringutil.ToSnakeCase(tbl.Name))
			it := make([]string, 0)
			for _, i := range tbl.Index {
				it = append(it, ":" + stringutil.ToSnakeCase(i))
			}
			is += strings.Join(it, ", ")
			fmt.Print(is)
			fmt.Println("], unique:true")
		}
		fmt.Println()
	}
}


func init() {
}

func main() {
    flag.Parse()

    var tables []table

    fn, _ := filepath.Abs(flag.Arg(0))
    fi,_  := os.Stat(fn)
    fset := token.NewFileSet()
    if fi.IsDir() {
        pkgs, _ := parser.ParseDir(fset, fn, nil, parser.ParseComments)
        for _, pkg := range pkgs {
            for _, file := range pkg.Files {
                tables = parseFile(fset, file, tables)
            }
        }
    } else {
        file, _ := parser.ParseFile(fset, fn, nil, parser.ParseComments)
        tables = parseFile(fset, file, tables)
    }

	putMigrate(tables)
}
