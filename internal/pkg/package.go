package pkg

// Simulates declarations typically coming from an internal package.

const InternalVersion = 42

var (
	DefaultName = "bot"
	MaxRetries  = 3
)

func Add(a, b int) int {
	return a + b
}

func Clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func Unused() string {
	return "this should be stripped"
}
