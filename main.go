package main

import (
	"github.com/bitlancer/freshbot/lib"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(lib.HandleRequest)
}
