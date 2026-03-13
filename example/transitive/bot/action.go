package bot

import "github.com/mrsombre/codingame-golang-merger/example/transitive/game"

// Move returns the result of stepping from pos in the given direction.
// Uses game.Point internally — must be rewritten to Point after merging.
func Move(pos game.Point, dx, dy int) game.Point {
	return game.Add(pos, game.Point{X: dx, Y: dy})
}
