package main

import (
	"os"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

func main() {
	os.Exit(boundarycli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
