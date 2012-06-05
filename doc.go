// Copyright 2012 Thorbjørn Willoch
// No rights reserved
//

/*

tago makes etags for go codes

		 Usage : 	tago [-f] [-o tagfile] gofiles....

		 By default the tags are package.func, package.type, and type.receiver .
		 If the -f flag is used the receivers get the tag package.type.receiver .

		 Typical usage : tago `find $GOPATH -name \*.go`
		      
*/
package documentation
