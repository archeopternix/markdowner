package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/archeopternix/markdowner"
)

func main() {
	input, err := os.ReadFile("testdata/sample.md")
	if err != nil {
		fmt.Fprintf(os.Stderr, "read sample.md: %v\n", err)
		os.Exit(1)
	}

	fmText, body := splitYAMLFrontmatter(string(input))
	md := markdowner.NewDefaultMarkdowner()
	fm, err := md.DecodeFrontmatter([]byte(fmText))
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode frontmatter: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Frontmatter:")
	fmt.Printf("  Author: %s\n", fm.Author)
	fmt.Printf("  Title: %s\n", fm.Title)
	fmt.Printf("  Subtitle: %s\n", fm.Subtitle)
	fmt.Printf("  Date: %s\n", fm.Date)
	fmt.Printf("  ChangedDate: %s\n", fm.ChangedDate)
	fmt.Printf("  OriginalDocument: %s\n", fm.OriginalDocument)
	fmt.Printf("  OriginalFormat: %s\n", fm.OriginalFormat)
	fmt.Printf("  Version: %s\n", fm.Version)
	fmt.Printf("  Language: %s\n", fm.Language)
	fmt.Printf("  Abstract: %s\n", fm.Abstract)
	fmt.Printf("  Keywords: %v\n", fm.Keywords)

	fmt.Println()
	fmt.Println("Markdown Body:")
	fmt.Println(body)
}

func splitYAMLFrontmatter(s string) (string, string) {
	s = strings.TrimPrefix(s, "\ufeff")
	if !strings.HasPrefix(s, "---") {
		return "", s
	}

	r := bufio.NewReader(strings.NewReader(s))
	first, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", s
	}
	if strings.TrimSpace(first) != "---" {
		return "", s
	}

	var fm strings.Builder
	for {
		line, e := r.ReadString('\n')
		if e != nil && e != io.EOF {
			return "", s
		}
		if strings.TrimSpace(line) == "---" {
			break
		}
		fm.WriteString(line)
		if e == io.EOF {
			return "", s
		}
	}

	body, _ := io.ReadAll(r)
	return fm.String(), string(body)
}
