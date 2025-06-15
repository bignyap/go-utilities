package main

import (
	"fmt"

	"github.com/bignyap/go-utilities/logger/api"
	"github.com/bignyap/go-utilities/logger/factory"
)

func main() {

	fmt.Println("Welcome to debugging")

	logger := factory.GetGlobalLogger()

	logger.Info("Calling some function",
		api.Field{
			Key:   "function",
			Value: "SomeFunction",
		},
	)
}
