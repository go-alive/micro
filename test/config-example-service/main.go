package main

import (
	"fmt"

	"github.com/go-alive/go-micro"
)

func main() {
	// New Service
	service := micro.NewService(
		micro.Name("go.micro.service.config-read"),
		micro.Version("latest"),
	)
	service.Init()

	// create a new config
	c := service.Options().Config

	// set a value
	fmt.Println("Value of key.subkey: ", c.Get("key", "subkey").String(""))
}
