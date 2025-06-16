package entity

type Entity interface {
	ID() string
	Width() float32
	Height() float32
}

// This file stores all possible block states into a TAG_List with gzip compressed.
//
//go:generate go run ./generator/main.go

type ID int
