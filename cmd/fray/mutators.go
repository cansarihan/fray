package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cansarihan/fray/internal/mutate"
)

func runMutators(args []string) error {
	flags := flag.NewFlagSet("mutators", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "MUTATOR\tWHAT IT SENDS")
	for _, m := range mutate.Mutators() {
		fmt.Fprintf(tw, "%s\t%s\n", m.Name(), m.Describe())
	}
	return tw.Flush()
}
