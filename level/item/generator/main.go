package main

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"go/format"
	"log"
	"os"
	"text/template"

	"git.konjactw.dev/falloutBot/go-mc/internal/generateutils"
	"git.konjactw.dev/falloutBot/go-mc/nbt"
)

//go:embed items.go.tmpl
var tempSource string

var temp = template.Must(template.
	New("item_template").
	Funcs(template.FuncMap{
		"UpperTheFirst": generateutils.UpperTheFirst,
		"ToGoTypeName":  generateutils.ToGoTypeName,
		"Generator":     func() string { return "generator/main.go" },
	}).
	Parse(tempSource),
)

type Item struct {
	Name  string
	Id    int
	Block string `nbt:"Block,omitempty"`
}

func main() {
	var states []Item
	readItems(&states)

	// generate go source file
	genSourceFile(states)
}

func readItems(states *[]Item) {
	// open block_states data file
	f, err := os.Open("items.nbt")
	if err != nil {
		log.Panic(err)
	}
	defer f.Close()

	r, err := gzip.NewReader(f)
	if err != nil {
		log.Panic(err)
	}

	// parse the nbt format
	if _, err := nbt.NewDecoder(r).Decode(states); err != nil {
		log.Panic(err)
	}
}

func genSourceFile(states []Item) {
	var source bytes.Buffer
	if err := temp.Execute(&source, states); err != nil {
		log.Panic(err)
	}

	formattedSource, err := format.Source(source.Bytes())
	if err != nil {
		panic(err)
	}

	err = os.WriteFile("items.go", formattedSource, 0o666)
	if err != nil {
		panic(err)
	}
	log.Print("Generated items.go")
}
