package alpha

type ExampleType struct {
	Name  string
	Value int
}

const ExampleConst = "alpha"

const ExampleUnusedConst = "unused"

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
