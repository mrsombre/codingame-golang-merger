package gamma

import (
	"github.com/mrsombre/codingame-golang-merger/example/nested/alpha"
	"github.com/mrsombre/codingame-golang-merger/example/nested/gamma/epsilon"
)

type ExampleTypeWithFunc struct {
	ID    int
	Value float64
}

func (s *ExampleTypeWithFunc) Read() float64 {
	return s.calibrate()
}

func (s *ExampleTypeWithFunc) calibrate() float64 {
	return epsilon.ExampleScale(s.Value, 105) / 100
}

type ExampleTypeUsingOther struct {
	Name string
}

func (m *ExampleTypeUsingOther) Check(s *ExampleTypeWithFunc) string {
	val := s.Read()
	if val > float64(alpha.ExampleVar) {
		return m.Name + ": HIGH"
	}
	return m.Name + ": OK"
}
