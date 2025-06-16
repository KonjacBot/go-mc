package sound

type Sound interface {
	sound()
	ID() string
}

// This file stores all possible block states into a TAG_List with gzip compressed.
//
//go:generate go run ./generator/main.go

type ID int
