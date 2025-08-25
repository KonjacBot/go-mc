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

//go:embed entities.go.tmpl
var tempSource string

var temp = template.Must(template.
	New("entity_template").
	Funcs(template.FuncMap{
		"UpperTheFirst": generateutils.UpperTheFirst,
		"ToGoTypeName":  generateutils.ToGoTypeName,
		"Generator":     func() string { return "generator/main.go" },
	}).
	Parse(tempSource),
)

type Entity struct {
	Name   string
	Id     int
	Width  float32
	Height float32
}

func main() {
	var entities []Entity
	readItems(&entities)

	// generate go source file
	genSourceFile(entities)
}

func readItems(states *[]Entity) {
	// open block_states data file
	f, err := os.Open("entities.nbt")
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

func genSourceFile(states []Entity) {
	var source bytes.Buffer
	if err := temp.Execute(&source, states); err != nil {
		log.Panic(err)
	}

	formattedSource, err := format.Source(source.Bytes())
	if err != nil {
		panic(err)
	}

	err = os.WriteFile("entities.go", formattedSource, 0o666)
	if err != nil {
		panic(err)
	}
	log.Print("Generated entities.go")
}
