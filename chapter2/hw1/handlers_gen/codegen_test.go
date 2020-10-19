package main

import (
	"testing"
)

func TestGenerateAPI(t *testing.T) {
	inputFile := "../api.go"
	outputFile := "../api_handlers.go"

	err := generateAPI(inputFile, outputFile)
	if err != nil {
		t.Fatal(err)
	}
}
