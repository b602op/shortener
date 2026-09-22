package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("start")
	os.Exit(1) // want "direct call to os.Exit in main is prohibited"
}

func helper() {
	os.Exit(2) // OK — не в main
}

func anotherFunc() {
	os.Exit(3) // OK — не в main
}
