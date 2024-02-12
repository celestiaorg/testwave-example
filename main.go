package main

import (
	"github.com/celestiaorg/testwave-example/testplan"
	"github.com/celestiaorg/testwave/cmd/testwave"
)

func main() {
	tw := testwave.New(&testplan.Playbook{})
	tw.Execute()
}
