package item

import (
	_ "embed"

	"github.com/Tnze/go-mc/level/block"
)

type Item interface {
	ID() string
}

type BlockItem interface {
	Block() block.Block
}

// This file stores all possible block states into a TAG_List with gzip compressed.
//
//go:generate go run ./generator/main.go
type ID int
