package main

import (
	"flag"
	"fmt"
	"os"

	"skills/arcgocore/internal/scaffold"
)

func main() {
	opts := scaffold.ExecScaffoldOptions{}
	fs := flag.NewFlagSet("execscaffold", flag.ExitOnError)
	fs.StringVar(&opts.Workdir, "workdir", "", "Absolute path of target workdir")
	fs.StringVar(&opts.TaskName, "task-name", "", "Orchestration case name")
	fs.StringVar(&opts.OutputDir, "output-dir", "", "Case output directory override")
	fs.Parse(os.Args[1:])
	if opts.Workdir == "" || opts.TaskName == "" {
		fmt.Fprintln(os.Stderr, "--workdir and --task-name are required")
		os.Exit(2)
	}
	caseDir, err := scaffold.RunExecScaffold(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Created exec case: %s\n", caseDir)
}
