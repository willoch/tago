/*
 Tago "Emacs etags for Go"
 Author: Thorbjørn Willoch
 Email: thwilloch@gmail.com
 Version: 1.0
 Based on work done by:
 	Alex Combas 2010
	 Initial release: January 03 2010
 Update Mai 22 2012
*/

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"unicode/utf8"
)

// Get working directory and set it for savePath flag default

func whereAmI() string {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting working directory: %s\n", err.Error())
		return ""
	}
	return dir + "/"
}

// Setup flag variables
var (
	saveDir = flag.String("d", whereAmI(),
				"Change save directory: -d=/path/to/my/tags/")
	tagsName           = flag.String("o",
		"TAGS", "Change TAGS name: -o=MyTagsFile")
	appendMode         = flag.Bool("a", false, "Append mode: -a")
	fset               *token.FileSet
	contentCurrentFile []string
)

type Tea struct {
	bytes.Buffer
}

func LineAndPos(lineno int) (line string, pos int) {
	for i := 0; i < lineno-1; i++ {
		pos += utf8.RuneCountInString(contentCurrentFile[i])
	}
	x := contentCurrentFile[lineno-1]
	line = x[:len(x)-1]
	return
}

// Writes a TAGS line to a Tea buffer

func (t *Tea) drink(leaf *ast.Ident, pkgname, rcvname string) {
	P := fset.Position(leaf.NamePos)
	n, l := leaf.String(), P.Line
	s, o := LineAndPos(P.Line)
	if rcvname != "" {
		pkgname = rcvname
	}
	fmt.Fprintf(t, "%s%s.%s%d,%d\n", s, pkgname, n, l, o)
}

// TAGS file is either appended or created, not overwritten.
func (t *Tea) savor() {
	location := fmt.Sprintf("%s%s", *saveDir, *tagsName)
	flag := os.O_WRONLY
	if *appendMode {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_CREATE | os.O_TRUNC
	}
	file, err := os.OpenFile(location, flag, 0666)
	if err != nil {
		fmt.Println(`Error opening file "`, location, `": `, err.Error())
		return
	}
	file.Write(t.Bytes())
	file.Close()
}

func theReceiver(leaf *ast.FuncDecl) (ret string) {
	if leaf.Recv != nil && len(leaf.Recv.List) > 0 {
		switch x := leaf.Recv.List[0].Type.(type) {
		case *ast.StarExpr:
			return x.X.(*ast.Ident).Name
		case *ast.Ident:
			return x.Name
		}
		fmt.Printf("Should not come here %#v\n", leaf.Recv.List[0].Type)
	}
	return
}

func readCurrentFile(name string) (ret []string) {
	ret = make([]string, 0, 2000)
	file, err := os.Open(name)
	if err != nil {
		fmt.Printf("Error opening file in readCurrentFile(%s): %s\n",
			name, err.Error())
		return nil
	}
	defer file.Close()
	r := bufio.NewReader(file)
	for {
		n, e := r.ReadString('\n')
		if e == nil {
			ret = append(ret, n)
			if len(n) == 0 {
				break
			}
		} else {
			if e != io.EOF {
				fmt.Printf("Error reading: %v\n", e)
			}
			break
		}
	}
	return
}


// Parses the source files given on the commandline,
// returns a TAGS chunk for each file
func brew() string {
	teaPot := Tea{}
	for i := 0; i < len(flag.Args()); i++ {
		fset = token.NewFileSet()
		teaCup := Tea{}
		ptree, perr := parser.ParseFile(fset, flag.Arg(i), nil, 0)
		if perr != nil {
			fmt.Println("Error parsing file ",
				flag.Arg(i), ": ", perr.Error())
			continue
		}
		contentCurrentFile = readCurrentFile(flag.Arg(i))
		pkgName := ptree.Name.Name
		for _, l := range ptree.Decls {
			switch leaf := l.(type) {
			case *ast.FuncDecl:
				teaCup.drink(leaf.Name, pkgName,
					theReceiver(leaf))
			case *ast.GenDecl:
				for _, c := range leaf.Specs {
					switch cell := c.(type) {
					case *ast.TypeSpec:
						teaCup.drink(cell.Name, pkgName, "")
					case *ast.ValueSpec:
						for _, atom := range cell.Names {
							teaCup.drink(atom, pkgName, "")
						}
					}
				}
			}
		}
		totalBytes := teaCup.Len()
		P := fset.Position(ptree.Pos())
		fmt.Fprintf(&teaPot, "\f\n%s,%d\n%s", P.Filename, totalBytes, &teaCup)
	}
	return teaPot.String()
}

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Printf(
			"Usage: %s [-a] [-h] [-d directory] [-o TagsFile] source.go ...\n",
			os.Args[0])
		return
	}
	tea := Tea{}
	fmt.Fprint(&tea, brew())

	// if the string is empty there were parsing errors, abort
	if tea.String() == "" {
		fmt.Println("Parsing errors experienced, aborting...")
	} else {
		tea.savor()
	}
}
