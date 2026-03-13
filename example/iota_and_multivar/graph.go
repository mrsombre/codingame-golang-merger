package main

// Bug 1 (cont): second iota block — must stay separate from direction's block.

type NodeType uint8

const (
	NodeNone NodeType = iota // 0
	NodeEdge                 // 1
	NodeTmp                  // 2
)

// Bug 2: multi-name var — merger must not duplicate the declaration.
var W, H int
