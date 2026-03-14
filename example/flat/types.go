package main

type ExampleType struct {
	Name  string
	Value int
}

type ExampleUnusedType struct {
	ID int
}

type ExampleTypeWithFunc struct {
	ID    int
	Value float64
}

func (s *ExampleTypeWithFunc) Read() float64 {
	return s.calibrate()
}

func (s *ExampleTypeWithFunc) calibrate() float64 {
	return s.Value * 1.05
}

type ExampleTypeUsingOther struct {
	Name string
}

func (m *ExampleTypeUsingOther) Check(s *ExampleTypeWithFunc) string {
	val := s.Read()
	if val > 100 {
		return m.Name + ": HIGH"
	}
	return m.Name + ": OK"
}

type ExampleIotaKind int

const (
	ExampleConstIotaA ExampleIotaKind = iota
	ExampleConstIotaB
	ExampleConstIotaC
)
