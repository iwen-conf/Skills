package main

import (
	"flag"
	"fmt"
	"os"

	"skills/arcgocore/internal/scaffold"
)

func main() {
	opts := scaffold.BuildScaffoldOptions{}
	fs := flag.NewFlagSet("buildscaffold", flag.ExitOnError)
	fs.StringVar(&opts.ProjectPath, "project-path", "", "Absolute path of target project")
	fs.StringVar(&opts.TaskName, "task-name", "", "Implementation task name")
	fs.StringVar(&opts.OutputDir, "output-dir", "", "Case output directory override")
	fs.Parse(os.Args[1:])
	if opts.ProjectPath == "" || opts.TaskName == "" {
		fmt.Fprintln(os.Stderr, "--project-path and --task-name are required")
		os.Exit(2)
	}
	caseDir, err := scaffold.RunBuildScaffold(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Created build case: %s\n", caseDir)
}
