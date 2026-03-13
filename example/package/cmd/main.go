package main

import (
	"fmt"

	"github.com/mrsombre/codingame-golang-merger/internal/pkg"
)

func main() {
	fmt.Println("Hello, world!")

	fmt.Println(pkg.Clamp(10, 0, 5))
}
