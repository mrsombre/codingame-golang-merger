package pkg

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Error("Expected 5")
	}
}

func TestClamp(t *testing.T) {
	if Clamp(10, 0, 5) != 5 {
		t.Error("Expected 5")
	}
	if Clamp(-1, 0, 5) != 0 {
		t.Error("Expected 0")
	}
	if Clamp(3, 0, 5) != 3 {
		t.Error("Expected 3")
	}
}
