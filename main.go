package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/bitlancer/freshbot/lib"
)

func main() {
	lambda.Start(lib.HandleRequest)
}
