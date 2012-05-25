/*
 Tago "Emacs etags for Go"
 Author: Thorbjørn Willoch
 Email: thwilloch@gmail.com
 Version: 1.0
 Based on work done by:
 	Alex Combas 2010
	 Initial release: January 03 2010
 Updated May 22 2012
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
	"log"
	"os"
	"strings"
	"unicode/utf8"
)

type lineAndPos struct {
	p int
	l string
}

var (
	fullTag        = flag.Bool("f", false, "make package.type.Ident tags for receivers")
	tagsName       = flag.String("o", "TAGS", "Change TAGS name: -o=MyTagsFile")
	appendMode     = flag.Bool("a", false, "Append mode: -a")
	fset           *token.FileSet
	contentCurrent []lineAndPos
)

type Buffer struct {
	bytes.Buffer
}

func LineAndPos(lineno int, ident string) (line string, pos int) {
	if lineno > len(contentCurrent) {
		return "Something Rotten! (probably //line declerations)", 0
	}
	v := contentCurrent[lineno-1]
	p := strings.Index(v.l, ident)
	if p == -1 {
		return v.l[:len(v.l)-1], v.p
	}
	return v.l[:p+len(ident)+1], v.p
}

// Writes a TAGS line to a Buffer buffer
func (t *Buffer) tagLine(leaf *ast.Ident, pkgname, rcvname string) {
	P := fset.Position(leaf.NamePos)
	n, l := leaf.String(), P.Line
	s, o := LineAndPos(P.Line, n)
	beforedot := pkgname
	if rcvname != "" {
		beforedot = rcvname
	}
	if rcvname != "" && *fullTag {
		fmt.Fprintf(t, "%s\177%s.%s.%s\001%d,%d\n", s, pkgname, rcvname, n, l, o)
	} else {
		fmt.Fprintf(t, "%s\177%s.%s\001%d,%d\n", s, beforedot, n, l, o)
	}
}

func getFile() *os.File {
	flag := os.O_WRONLY | os.O_CREATE
	if *appendMode {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	f, e := os.OpenFile(*tagsName, flag, 0666)
	if e != nil {
		log.Fatal(e)
	}
	return f
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

func readCurrentFile(name string) (ret []lineAndPos) {
	ret = make([]lineAndPos, 0, 2000)
	file, err := os.Open(name)
	if err != nil {
		log.Fatalf("Error opening file in readCurrentFile(%s): %s\n", name, err)
	}
	defer file.Close()
	r := bufio.NewReader(file)
	p := 0
	for {
		switch n, e := r.ReadString('\n'); e {
		case io.EOF:
			return
		case nil:
			ret = append(ret, lineAndPos{p, n})
			p += utf8.RuneCountInString(n)
		default:
			fmt.Println("Error reading ", file, " : ", e)
			return
		}
	}
}

// Parses the source files given on the commandline,
// returns a TAGS chunk for each file
func iterateGenDecl(leaf *ast.GenDecl, pkgName string, fileBuffer *Buffer) {
	for _, c := range leaf.Specs {
		switch cell := c.(type) {
		case *ast.TypeSpec:
			fileBuffer.tagLine(cell.Name, pkgName, "")
		case *ast.ValueSpec:
			for _, atom := range cell.Names {
				fileBuffer.tagLine(atom, pkgName, "")
			}
		}
	}
}

func iterateParsedFile(ptree *ast.File, fileBuffer *Buffer) {
	pkgName := ptree.Name.Name
	for _, l := range ptree.Decls {
		switch leaf := l.(type) {
		case *ast.FuncDecl:
			fileBuffer.tagLine(leaf.Name, pkgName, theReceiver(leaf))
		case *ast.GenDecl:
			iterateGenDecl(leaf, pkgName, fileBuffer)
		}
	}
}

func parseFiles(outfile *os.File) {
	for _, file := range flag.Args() {
		fset = token.NewFileSet()
		ptree, perr := parser.ParseFile(fset, file, nil, 0)
		if perr != nil {
			log.Println("Error parsing file ", file, ": ", perr)
			continue
		}
		contentCurrent = readCurrentFile(file)

		fileBuffer := Buffer{}
		iterateParsedFile(ptree, &fileBuffer)
		totalBytes := fileBuffer.Len()
		fmt.Fprintf(outfile, "\f\n%s,%d\n%s", file, totalBytes, &fileBuffer)
	}
}
func init() {
	log.SetFlags(0)
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatalf("Usage: %s [-f] [-a] [-h] [-o TagsFile] source.go ...\n",
			os.Args[0])
	}
}
func nothing()

var noe int

func main() {
	f := getFile()
	defer f.Close()
	parseFiles(f)
}
