package item

import (
	_ "embed"

	"git.konjactw.dev/falloutBot/go-mc/level/block"
)

type Item interface {
	ID() ID
	Name() string
}

type BlockItem interface {
	Block() block.Block
}

// This file stores all possible block states into a TAG_List with gzip compressed.
//
//go:generate go run ./generator/main.go
type ID int
