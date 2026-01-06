package main

import (
	"context"
	"fmt"

	"github.com/bignyap/go-utilities/logger/api"
	"github.com/bignyap/go-utilities/logger/factory"
)

func main() {

	fmt.Println("Welcome to debugging")

	logger := factory.GetGlobalLogger()

	logger.Info(context.Background(), "Calling some function",
		api.Field{
			Key:   "function",
			Value: "SomeFunction",
		},
	)
}
