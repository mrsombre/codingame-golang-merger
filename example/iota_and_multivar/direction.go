package main

// Bug 1: iota constants — merger must preserve each const block separately,
// otherwise iota values get scrambled when blocks are combined.

type Direction int

const (
	DirUp    Direction = iota // 0
	DirRight                  // 1
	DirDown                   // 2
	DirLeft                   // 3
	DirNone                   // 4
)
