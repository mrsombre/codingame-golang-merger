package game

type Point struct {
	X int
	Y int
}

func Add(a, b Point) Point {
	return Point{X: a.X + b.X, Y: a.Y + b.Y}
}
