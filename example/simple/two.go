package main

import (
	"fmt"
)

var twoName = "2"

type two struct {
	c, d string
}

func settwo() *two {
	fmt.Println(twoName)

	return &two{"3", "4"}
}
