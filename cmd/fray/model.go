package main

import (
	"errors"
	"flag"
	"fmt"

	"github.com/cansarihan/fray/internal/model"
)

func runModel(args []string) error {
	flags := flag.NewFlagSet("model", flag.ContinueOnError)
	path := flags.String("model", "", "model file to validate")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *path == "" {
		return errors.New("model needs -model")
	}
	m, err := model.Load(*path)
	if err != nil {
		return err
	}
	fmt.Println(m)
	nodes, edges := m.Graph()
	fmt.Printf("\nstate graph (%d steps):\n", len(nodes))
	for _, e := range edges {
		fmt.Printf("  %s -> %s\n", e.From, e.To)
	}
	return nil
}
