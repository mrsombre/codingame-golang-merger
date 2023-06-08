package main

import (
	"fmt"
)

const oneName = "1"

type One struct {
	a, b int
}

func one() *One {
	fmt.Println(oneName)

	return &One{1, 2}
}
