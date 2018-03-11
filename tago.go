/*
 Tago "Emacs etags for Go"
 Author: Thorbjørn Willoch
 Email: thwilloch@gmail.com
 Version: 1.0
 Based on work done by:
 	Alex Combas 2010
	 Initial release: January 03 2010
 Updated March 9 2018
Changes from Jon Dilley
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
	fullTag    = flag.Bool("f", false, "make package.type.Ident tags for receivers")
	tagsName   = flag.String("o", "TAGS", "Change TAGS name")
	appendMode = flag.Bool("a", false, "Append mode ")
	fileList   = flag.String("i", "", "List of filenames to tagify, default empty reads Args")
)

type buffer struct {
	bytes.Buffer
	file           string
	fset           *token.FileSet
	contentCurrent []lineAndPos
}

func (t *buffer) lineANDpos(lineno int, ident string) (line string, pos int) {
	if lineno > len(t.contentCurrent) {
		return "Something Rotten! (probably //line declerations)", 0
	}
	v := t.contentCurrent[lineno-1]
	p := strings.Index(v.l, ident)
	if p == -1 {
		return v.l[:len(v.l)-1], v.p
	}
	return v.l[:p+len(ident)+1], v.p
}

// Writes a TAGS line to a buffer buffer
func (t *buffer) tagLine(leaf *ast.Ident, pkgname, rcvname string) {
	P := t.fset.Position(leaf.NamePos)
	n, l := leaf.String(), P.Line
	s, o := t.lineANDpos(P.Line, n)
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
func iterateGenDecl(leaf *ast.GenDecl, pkgName string, fileBuffer *buffer) {
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

func iterateParsedFile(ptree *ast.File, fileBuffer *buffer) {
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

func parseFile(file string) *buffer {
	fileBuffer := buffer{file: file, fset: token.NewFileSet()}
	ptree, perr := parser.ParseFile(fileBuffer.fset, file, nil, 0)
	if perr != nil {
		log.Println(perr)
		return nil
	}
	fileBuffer.contentCurrent = readCurrentFile(file)
	iterateParsedFile(ptree, &fileBuffer)
	return &fileBuffer
}

func parseFiles(outfile *os.File) {
	c := make(chan *buffer, 16)
	f := make(chan string, 16)
	var filenames []string
	if len(*fileList) > 0 {
		fnf, err := os.OpenFile(*fileList, os.O_RDONLY, 0)
		if err != nil {
			log.Fatal(err)
		}
		defer fnf.Close()
		s := bufio.NewScanner(fnf)
		for s.Scan() {
			filenames = append(filenames, s.Text())
		}
	}
	// Append any command line arguments
	for _, a := range flag.Args() {
		filenames = append(filenames, a)
	}
	for i := 0; i != 8; i++ {
		go func() {
			for file := range f {
				c <- parseFile(file)
			}
		}()
	}
	go func() {
		for _, file := range filenames {
			f <- file
		}
		close(f)
	}()

	for i, n := 0, len(filenames); i != n; i++ {
		if fileBuffer := <-c; fileBuffer != nil {
			totalBytes := fileBuffer.Len()
			fmt.Fprintf(outfile, "\f\n%s,%d\n%s", fileBuffer.file,
				totalBytes, fileBuffer)
		}
	}
}

func init() {
	log.SetFlags(0)
	flag.Parse()
	if flag.NArg() == 0 && len(*fileList) == 0 {
		log.Fatalf("Usage: %s  [-f] [-a] [-h] [-o TagsFile] " +
		"[-i ListOfFilesFile] source.go ...\n", os.Args[0])
	}
}

func main() {
	f := getFile()
	defer f.Close()
	parseFiles(f)
}
