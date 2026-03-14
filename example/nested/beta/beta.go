package beta

import "github.com/mrsombre/codingame-golang-merger/example/nested/alpha"

// ExampleFunc has same name as alpha.ExampleFunc — prefix mapping resolves collision.
func ExampleFunc(a alpha.ExampleType, n int) alpha.ExampleType {
	return alpha.ExampleFunc(a, alpha.ExampleType{Name: "b", Value: n})
}

func ExampleUnusedFunc() int {
	return alpha.ExampleVar + 1
}
