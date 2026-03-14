package main

import "fmt"

func main() {
	v := ExampleFunc(1, 2)
	fmt.Println(v)
	fmt.Println(ExampleConst)
	fmt.Println(ExampleConstIotaA, ExampleConstIotaB, ExampleConstIotaC)
	fmt.Println(ExampleVar)
	fmt.Println(ExampleMultiVarA, ExampleMultiVarB)

	s := ExampleType{Name: "test", Value: 42}
	fmt.Println(s.Name, s.Value)

	exWithFunc := &ExampleTypeWithFunc{ID: 1, Value: 95.0}
	exUsingOther := &ExampleTypeUsingOther{Name: "main"}
	fmt.Println(exUsingOther.Check(exWithFunc))
}
