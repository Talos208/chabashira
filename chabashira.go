package main

import (
	"flag"
	"fmt"
	"github.com/naoina/go-stringutil"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
		case "bool":
			col.Type = "boolean"
			col.Opt["null"] = "false"
		case "int", "int64", "uint", "uint64":
			col.Type = "integer"
			col.Opt["null"] = "false"
			col.Opt["limit"] = "8"
		case "int32", "uint32":
			col.Type = "integer"
			col.Opt["null"] = "false"
			col.Opt["limit"] = "4"
		case "int16", "uint16":
			col.Type = "integer"
			col.Opt["null"] = "false"
			col.Opt["limit"] = "2"
		case "int8","uint8","byte":
			col.Type = "integer"
			col.Opt["null"] = "false"
			col.Opt["limit"] = "1"
		case "string":
			col.Type = "string"
			col.Opt["null"] = "false"
		case "byte[]":
			col.Type = "binary"
		case "float", "float64":
			col.Type = "float"
			col.Opt["null"] = "false"
		case "Time":
			col.Type = "timestamp" // TODO Sometime, it's suite to use datetime.
			col.Opt["null"] = "true"
			col.Opt["default"] = "0"
		case "NullBool":
			col.Type = "boolean"
		case "NullInt64":
			col.Type = "integer"
		case "NullFloat64":
			col.Type = "float"
		case "NullString":
			col.Type = "string"
		default:
			log.Print(reflect.TypeOf(tp).String(), tp)
		}

		if !fld.Names[0].IsExported() {
			continue
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
					case "-": // no target.
						continue EachField
					case "pk": // primary key
						tbl.Pk = col.Name
					case "unique": // unique constraint
						tbl.Index = append(tbl.Index, col.Name)
					}
				case "size": // size constraint
					col.Opt["limit"] = kv[1]
				case "default": // default value
					col.Opt["default"] = kv[1]
				case "column": // column name
					col.Name = kv[1]
				case "refer": // reference to another table
					col.Type = "references"
					delete(col.Opt, "null")
					if len(kv[1]) > 0 {
						col.Name = kv[1]
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
func putMigrate(tables []table, w io.Writer) {
	for _, tbl := range tables {
		ixFlg := true
		if len(tbl.Index) == 1 {
			ixFlg = false
		}

		fmt.Fprintf(w, "create_table '%s'", stringutil.ToSnakeCase(tbl.Name))
		if len(tbl.Pk) == 0 {
			fmt.Fprint(w, ", id:false")
		} else if tbl.Pk != "Id" {
			fmt.Fprint(w, ", primary_key:'", stringutil.ToSnakeCase(tbl.Pk),"'")
		}
		fmt.Fprint(w, " do |t|\n")
		for _, col := range tbl.Columns {
			name := col.Name
			if name == tbl.Pk {
				continue
			}
			if col.Type == "references" {
				name = strings.TrimSuffix(name, "Id")
			}
			cs := fmt.Sprintf("t.%s :%s", col.Type, stringutil.ToSnakeCase(name))
			if !ixFlg && name == tbl.Index[0] {
				cs = cs + ", unique:true"
			}
			if len(col.Opt["null"]) > 0 {
				cs = cs + ", null:" + col.Opt["null"]
			}
			if len(col.Opt["default"]) > 0 {
				cs = cs + ", default:" + col.Opt["default"]
			}
			if len(col.Opt["limit"]) > 0 {
				cs = cs + ", limit:" + col.Opt["limit"]
			}
			fmt.Fprintln(w, " ", cs)
		}
		fmt.Fprintln(w, "end")
		if len(tbl.Index) > 0 {
			is := fmt.Sprintf("add_index :%s, [", stringutil.ToSnakeCase(tbl.Name))
			it := make([]string, 0)
			for _, i := range tbl.Index {
				it = append(it, ":"+stringutil.ToSnakeCase(i))
			}
			is += strings.Join(it, ", ")
			fmt.Fprint(w, is)
			fmt.Fprintln(w, "], unique:true")
		}
		fmt.Fprintln(w)
	}
}

func putNames(tables []table, pkg string, w io.Writer) {
	fmt.Fprintln(w, "package", pkg)
	for _, tbl := range tables {
		fmt.Fprintln(w, "// ", tbl.Name)
		for _, col := range tbl.Columns {
			fmt.Fprintln(w, "func (*", tbl.Name, ") ",stringutil.ToSnakeCase(col.Name),"() string {")
			fmt.Fprint(w, "\treturn \"")
			fmt.Fprint(w, stringutil.ToSnakeCase(col.Name))
			fmt.Fprintln(w, "\"")
			fmt.Fprintln(w, "}")
		}
	}
}

var (
	schmOut settableWriter = settableWriter{os.Stdout}
	nmOut settableWriter
	nmPkg string
)

type settableWriter struct {
	out io.Writer
}

func (*settableWriter) String() string {
	return ""
}

func (s *settableWriter) Set(fn string) error {
	f, err := os.Create(fn)
	if f == nil || err != nil {
		log.Print(err)
		return err
	}
	s.out = f
	return nil
}

func (s settableWriter) Write(p []byte) (int, error) {
	if s.out == nil {
		log.Print("Not initialized.")
		return 0, nil
	}
	return s.out.Write(p)
}

func (s settableWriter) IsWriteble() bool {
	return s.out != nil
}

func init() {
	flag.Var(&schmOut, "o", "Output schema file path.")
	flag.Var(&nmOut, "n", "Output name file path.")
	flag.StringVar(&nmPkg, "p", "main", "Package name for generating files.")
}

func main() {
	flag.Parse()

	var tables []table

	fn, _ := filepath.Abs(flag.Arg(0))
	fi, err := os.Stat(fn)
	if err != nil {
		log.Panic(err)
	}
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

	putMigrate(tables, schmOut)

	if nmOut.IsWriteble() {
		putNames(tables, nmPkg, nmOut)
	}
}
