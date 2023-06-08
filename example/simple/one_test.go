package main

import (
	"testing"
)

func TestOne(t *testing.T) {
	if one().a != 1 {
		t.Error("Expected 1, got ", one().a)
	}
}
