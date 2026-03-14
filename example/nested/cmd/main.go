package main

import (
	"fmt"

	"github.com/mrsombre/codingame-golang-merger/example/nested/alpha"
	"github.com/mrsombre/codingame-golang-merger/example/nested/beta"
	"github.com/mrsombre/codingame-golang-merger/example/nested/gamma"
)

func main() {
	a := alpha.ExampleType{Name: "x", Value: 1}
	b := alpha.ExampleFunc(a, alpha.ExampleType{Name: "y", Value: 2})
	fmt.Println(b.Name, b.Value)

	c := beta.ExampleFunc(a, 10)
	fmt.Println(c.Name, c.Value)

	fmt.Println(alpha.ExampleConst)

	exWithFunc := &gamma.ExampleTypeWithFunc{ID: 1, Value: 95.0}
	exUsingOther := &gamma.ExampleTypeUsingOther{Name: "main"}
	fmt.Println(exUsingOther.Check(exWithFunc))
}
