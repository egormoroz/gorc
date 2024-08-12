package main

import (
	"fmt"
	"github.com/egormoroz/gorc/internal/server"
)

func main() {
	if err := server.Run("localhost:1337"); err != nil {
		fmt.Println("Something went wrong:", err)
	}
}
