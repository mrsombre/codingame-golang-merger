package main

import (
	"fmt"

	"github.com/mrsombre/codingame-golang-merger/example/transitive/bot"
	"github.com/mrsombre/codingame-golang-merger/example/transitive/game"
)

func main() {
	start := game.Point{X: 1, Y: 2}
	end := bot.Move(start, 3, 4)
	fmt.Println(end.X, end.Y) // 4 6
}
