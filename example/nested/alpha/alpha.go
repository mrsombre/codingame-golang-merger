package alpha

type ExampleType struct {
	Name  string
	Value int
}

const ExampleConst = "alpha"

const ExampleUnusedConst = "unused"

const (
	DirUp = iota
	DirRight
	DirDown
	DirLeft
)

var DirDelta = [4][2]int{
	DirUp:    {0, -1},
	DirRight: {1, 0},
	DirDown:  {0, 1},
	DirLeft:  {-1, 0},
}

var ExampleVar = 10

func ExampleFunc(a, b ExampleType) ExampleType {
	return ExampleType{
		Name:  a.Name + b.Name,
		Value: a.Value + b.Value,
	}
}

func ExampleUnusedFunc() string {
	return "drop me"
}
