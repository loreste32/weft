package weft

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/loreste/weft/internal/parse"
)

// LintIssue is a single lint finding.
type LintIssue struct {
	File     string
	Line     int
	Col      int
	Severity string // "error", "warning", "info"
	Rule     string
	Message  string
}

// LintReport is the result of linting.
type LintReport struct {
	Issues []LintIssue
	Files  int
	Errors int
	Warns  int
	Infos  int
}

// Lint runs static analysis on Weft source files.
func Lint(paths []string) (*LintReport, error) {
	if len(paths) == 0 {
		paths = []string{"."}
	}

	var files []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			filepath.Walk(p, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if fi.IsDir() {
					base := filepath.Base(path)
					if base == "vendor" || base == ".git" || base == "node_modules" {
						return filepath.SkipDir
					}
					return nil
				}
				if strings.HasSuffix(path, ".weft") {
					files = append(files, path)
				}
				return nil
			})
		} else {
			files = append(files, p)
		}
	}

	report := &LintReport{Files: len(files)}
	for _, f := range files {
		issues := lintFile(f)
		for _, issue := range issues {
			report.Issues = append(report.Issues, issue)
			switch issue.Severity {
			case "error":
				report.Errors++
			case "warning":
				report.Warns++
			case "info":
				report.Infos++
			}
		}
	}
	sort.Slice(report.Issues, func(i, j int) bool {
		if report.Issues[i].File != report.Issues[j].File {
			return report.Issues[i].File < report.Issues[j].File
		}
		return report.Issues[i].Line < report.Issues[j].Line
	})
	return report, nil
}

func lintFile(path string) []LintIssue {
	src, err := os.ReadFile(path)
	if err != nil {
		return []LintIssue{{File: path, Line: 1, Severity: "error", Rule: "read", Message: err.Error()}}
	}

	content := string(src)
	lines := strings.Split(content, "\n")
	var issues []LintIssue

	// parse check
	_, perrs := parse.ParseFile(path, content)
	if perrs.HasErrors() {
		for _, e := range perrs {
			if e.Severity == 0 { // error severity
				continue
			}
			issues = append(issues, LintIssue{
				File:     path,
				Line:     e.Pos.Line,
				Col:      e.Pos.Column,
				Severity: "error",
				Rule:     "parse",
				Message:  e.Message,
			})
		}
		if len(issues) == 0 {
			// fallback: report the whole list
			issues = append(issues, LintIssue{
				File: path, Line: 1, Severity: "error", Rule: "parse",
				Message: perrs.Error(),
			})
		}
		return issues
	}

	// line-level lint checks
	for i, line := range lines {
		lineNum := i + 1

		// trailing whitespace
		if len(line) > 0 && (line[len(line)-1] == ' ' || line[len(line)-1] == '\t') {
			issues = append(issues, LintIssue{
				File: path, Line: lineNum, Severity: "warning",
				Rule: "trailing-whitespace", Message: "trailing whitespace",
			})
		}

		// line too long (120 chars)
		if len(line) > 120 {
			issues = append(issues, LintIssue{
				File: path, Line: lineNum, Severity: "info",
				Rule: "line-length", Message: fmt.Sprintf("line is %d characters (max 120)", len(line)),
			})
		}

		// tab indentation (should be spaces)
		trimmed := strings.TrimLeft(line, " ")
		if len(trimmed) > 0 && trimmed[0] == '\t' {
			issues = append(issues, LintIssue{
				File: path, Line: lineNum, Severity: "warning",
				Rule: "no-tabs", Message: "use spaces for indentation, not tabs",
			})
		}

		// empty Result return without Ok/Err
		if strings.Contains(line, "return null") && !strings.Contains(line, "//") {
			issues = append(issues, LintIssue{
				File: path, Line: lineNum, Severity: "warning",
				Rule: "null-return", Message: "returning null — consider Ok(unit) or Err(...) for Result functions",
			})
		}

		// bare print instead of say
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, "println(") && !strings.Contains(line, "//") {
			issues = append(issues, LintIssue{
				File: path, Line: lineNum, Severity: "info",
				Rule: "prefer-say", Message: "prefer say() over println()",
			})
		}

		// TODO/FIXME/HACK comments
		upper := strings.ToUpper(line)
		for _, tag := range []string{"TODO", "FIXME", "HACK", "XXX"} {
			if strings.Contains(upper, "// "+tag) || strings.Contains(upper, "//"+tag) {
				issues = append(issues, LintIssue{
					File: path, Line: lineNum, Severity: "info",
					Rule: "todo", Message: fmt.Sprintf("found %s comment", tag),
				})
			}
		}
	}

	// file-level checks
	if !strings.Contains(content, "fn main") && !strings.Contains(content, "fn test_") &&
		!strings.Contains(content, "pub fn") && !strings.HasSuffix(path, "_test.weft") {
		issues = append(issues, LintIssue{
			File: path, Line: 1, Severity: "warning",
			Rule: "no-entry", Message: "file has no fn main, pub fn, or test functions",
		})
	}

	// check for unused imports (simple heuristic)
	for i, line := range lines {
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, "use ") && !strings.HasPrefix(stripped, "use \"./") {
			pkg := strings.TrimPrefix(stripped, "use ")
			pkg = strings.TrimSpace(pkg)
			if pkg != "" && !strings.Contains(pkg, "\"") {
				// check if the package name appears elsewhere in the file
				found := false
				for j, other := range lines {
					if j == i {
						continue
					}
					if strings.Contains(other, pkg+".") {
						found = true
						break
					}
				}
				if !found {
					issues = append(issues, LintIssue{
						File: path, Line: i + 1, Severity: "warning",
						Rule: "unused-import", Message: fmt.Sprintf("import %q may be unused", pkg),
					})
				}
			}
		}
	}

	return issues
}

// PrintLintReport prints the lint report and returns exit code.
func PrintLintReport(rep *LintReport) int {
	for _, issue := range rep.Issues {
		rel := issue.File
		if wd, err := os.Getwd(); err == nil {
			if r, err := filepath.Rel(wd, issue.File); err == nil {
				rel = r
			}
		}
		prefix := "info"
		switch issue.Severity {
		case "error":
			prefix = "ERROR"
		case "warning":
			prefix = "WARN"
		}
		loc := fmt.Sprintf("%s:%d", rel, issue.Line)
		if issue.Col > 0 {
			loc = fmt.Sprintf("%s:%d:%d", rel, issue.Line, issue.Col)
		}
		fmt.Printf("%-5s  %-30s  [%s] %s\n", prefix, loc, issue.Rule, issue.Message)
	}

	if len(rep.Issues) > 0 {
		fmt.Println()
	}
	fmt.Printf("weft lint  %d files, %d errors, %d warnings, %d info\n",
		rep.Files, rep.Errors, rep.Warns, rep.Infos)

	if rep.Errors > 0 {
		return 1
	}
	return 0
}
