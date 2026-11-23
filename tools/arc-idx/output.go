package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// emitSearch renders a search result as json or ai-md.
func emitSearch(res *searchResult, format string) error {
	if format == "json" {
		return emitRawJSON(json.MarshalIndent(res, "", "  "))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## search %q", res.Query)
	if res.Profile != "" {
		fmt.Fprintf(&b, " (profile: %s)", res.Profile)
	}
	fmt.Fprintf(&b, "\n%d files, %d matches, %s, engine=%s\n\n",
		res.FileCount, res.MatchCount, res.Duration, res.Engine)
	if res.FileCount == 0 {
		b.WriteString("No matches. Try a broader query, another profile, or `arc idx doctor` if results look incomplete.\n")
		fmt.Print(b.String())
		return nil
	}
	for _, f := range res.Files {
		fmt.Fprintf(&b, "### %s", f.Path)
		if f.Language != "" {
			fmt.Fprintf(&b, " (%s)", f.Language)
		}
		b.WriteString("\n```\n")
		if len(f.Matches) == 0 {
			b.WriteString("(filename match)\n")
		}
		for _, mt := range f.Matches {
			for i, c := range mt.Before {
				fmt.Fprintf(&b, "%d  %s\n", mt.Line-len(mt.Before)+i, c)
			}
			fmt.Fprintf(&b, "%d: %s\n", mt.Line, mt.Text)
			for i, c := range mt.After {
				fmt.Fprintf(&b, "%d  %s\n", mt.Line+1+i, c)
			}
		}
		b.WriteString("```\n\n")
	}
	fmt.Print(b.String())
	return nil
}
