/*
Sherpaclient calls Sherpa API functions and prints Sherpa API documentation from the command-line.

Example:

	sherpaclient -doc https://sherpa.irias.nl/example/

	sherpaclient -doc https://sherpa.irias.nl/example/ sum

	sherpaclient https://sherpa.irias.nl/example/ sum 1 1

The parameters to a function must be valid JSON. Don't forget to quote the double quotes of your JSON strings!

	Usage: sherpaclient [options] baseURL function [param ...]
	  -doc
		show documentation for all functions or single function if specified
	  -info
		show the API descriptor
*/
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"bitbucket.org/mjl/sherpa"
)

var (
	printDoc  = flag.Bool("doc", false, "show documentation for all functions or single function if specified")
	printInfo = flag.Bool("info", false, "show the API descriptor")
)

func main() {
	log.SetPrefix("sherpaclient: ")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: sherpaclient [options] baseURL function [param ...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(2)
	}

	url := args[0]
	args = args[1:]

	if *printDoc {
		if len(args) > 1 {
			flag.Usage()
			os.Exit(2)
		}
		doc(url, args)
		return
	}
	if *printInfo {
		if len(args) != 0 {
			flag.Usage()
			os.Exit(2)
		}
		info(url)
		return
	}

	if len(args) < 1 {
		flag.Usage()
		os.Exit(2)
	}
	function := args[0]
	args = args[1:]
	params := make([]interface{}, len(args))
	for i, arg := range args {
		err := json.Unmarshal([]byte(arg), &params[i])
		if err != nil {
			log.Fatalf("error parsing parameter %v: %s\n", arg, err)
		}
	}

	c, err := sherpa.NewClient(url, []string{})
	if err != nil {
		log.Fatal(err)
	}
	var result interface{}
	err = c.Call(&result, function, params...)
	if err != nil {
		switch serr := err.(type) {
		case *sherpa.Error:
			if serr.Code != "" {
				log.Fatalf("error %v: %s", serr.Code, serr.Message)
			}
		}
		log.Fatalf("error: %s", err)
	}
	err = json.NewEncoder(os.Stdout).Encode(&result)
	if err != nil {
		log.Fatal(err)
	}
}

func info(url string) {
	c, err := sherpa.NewClient(url, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Id: %s\n", c.Id)
	fmt.Printf("Title: %s\n", c.Title)
	fmt.Printf("Version: %s\n", c.Version)
	fmt.Printf("BaseURL: %s\n", c.BaseURL)
	fmt.Printf("SherpaVersion: %d\n", c.SherpaVersion)
	fmt.Printf("Functions:\n")
	for _, fn := range c.Functions {
		fmt.Printf("- %s\n", fn)
	}
}

func doc(url string, args []string) {
	c, err := sherpa.NewClient(url, nil)
	if err != nil {
		log.Fatal(err)
	}

	var doc sherpa.Doc
	cerr := c.Call(&doc, "_docs")
	if cerr != nil {
		log.Fatalf("fetching documentation: %s", cerr)
	}

	if len(args) == 1 {
		printFunction(&doc, args[0])
	} else {
		printDocs(&doc)
	}
}

func printFunction(doc *sherpa.Doc, function string) {
	for _, fn := range doc.Functions {
		if fn.Name == function {
			fmt.Println(fn.Text)
		}
	}
	for _, subDoc := range doc.Sections {
		printFunction(subDoc, function)
	}
}

func printDocs(doc *sherpa.Doc) {
	fmt.Printf("# %s\n\n%s\n\n", doc.Title, doc.Text)
	for _, fnDoc := range doc.Functions {
		fmt.Printf("# %s()\n%s\n\n", fnDoc.Name, fnDoc.Text)
	}
	for _, subDoc := range doc.Sections {
		printDocs(subDoc)
	}
	fmt.Println("")
}
