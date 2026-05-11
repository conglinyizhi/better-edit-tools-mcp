package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type commitItem struct {
	Type     string
	Scope    string
	Subject  string
	Raw      string
	Hash     string
	Breaking bool
}

var conventionalCommitRE = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9-]*)(?:\(([^)]+)\))?(!)?:\s*(.+)$`)

var sectionTitles = map[string]string{
	"feat":     "Features",
	"fix":      "Bug Fixes",
	"docs":     "Documentation",
	"test":     "Tests",
	"refactor": "Refactoring",
	"perf":     "Performance",
	"build":    "Build",
	"ci":       "CI",
	"style":    "Style",
	"chore":    "Chores",
	"revert":   "Reverts",
	"other":    "Other Changes",
}

var sectionOrder = []string{
	"feat",
	"fix",
	"docs",
	"refactor",
	"perf",
	"test",
	"build",
	"ci",
	"style",
	"chore",
	"revert",
	"other",
}

func main() {
	ref := "HEAD"
	if len(os.Args) > 1 && strings.TrimSpace(os.Args[1]) != "" {
		ref = strings.TrimSpace(os.Args[1])
	}

	prev, err := previousTag(ref)
	must(err)

	rangeSpec := ref
	if prev != "" {
		rangeSpec = prev + ".." + ref
	}

	commits, err := gitLog(rangeSpec)
	must(err)

	items := make([]commitItem, 0, len(commits))
	for _, line := range commits {
		item := parseCommit(line)
		items = append(items, item)
	}

	var breaking []commitItem
	sections := make(map[string][]commitItem)
	for _, item := range items {
		if item.Breaking {
			breaking = append(breaking, item)
			continue
		}
		sections[item.sectionKey()] = append(sections[item.sectionKey()], item)
	}

	var out strings.Builder
	fmt.Fprintf(&out, "## better-edit-tools %s\n\n", ref)

	wroteSection := false
	if len(breaking) > 0 {
		writeSection(&out, "Breaking Changes", breaking, true)
		wroteSection = true
	}

	for _, key := range sectionOrder {
		items := sections[key]
		if len(items) == 0 {
			continue
		}
		if wroteSection {
			out.WriteByte('\n')
		}
		writeSection(&out, sectionTitles[key], items, false)
		wroteSection = true
	}

	if !wroteSection {
		out.WriteString("- No commits found for this release.\n")
	}

	os.Stdout.WriteString(out.String())
}

func must(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func previousTag(ref string) (string, error) {
	out, err := runGit("describe", "--tags", "--abbrev=0", ref+"^")
	if err != nil {
		return "", nil
	}

	current := strings.TrimSpace(ref)
	for _, line := range splitLines(out) {
		tag := strings.TrimSpace(line)
		if tag == "" || tag == current {
			continue
		}
		return tag, nil
	}
	return "", nil
}

func gitLog(rangeSpec string) ([]string, error) {
	out, err := runGit("log", "--no-merges", "--format=%s%x1f%h", rangeSpec)
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

func splitLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func parseCommit(line string) commitItem {
	parts := strings.SplitN(line, "\x1f", 2)
	subject := strings.TrimSpace(parts[0])
	hash := ""
	if len(parts) == 2 {
		hash = strings.TrimSpace(parts[1])
	}

	item := commitItem{
		Raw:     subject,
		Subject: subject,
		Hash:    hash,
	}

	lower := strings.ToLower(subject)
	if strings.HasPrefix(lower, "revert:") {
		item.Type = "revert"
		item.Subject = strings.TrimSpace(subject[len("revert:"):])
		return item
	}
	if strings.HasPrefix(subject, "Revert ") {
		item.Type = "revert"
		return item
	}

	matches := conventionalCommitRE.FindStringSubmatch(subject)
	if matches == nil {
		item.Type = "other"
		return item
	}

	item.Type = normalizeType(strings.ToLower(matches[1]))
	item.Scope = strings.TrimSpace(matches[2])
	item.Breaking = matches[3] == "!"
	item.Subject = strings.TrimSpace(matches[4])
	if item.Subject == "" {
		item.Subject = item.Raw
	}
	return item
}

func normalizeType(t string) string {
	switch t {
	case "feature":
		return "feat"
	case "bugfix", "hotfix", "repair":
		return "fix"
	case "tests":
		return "test"
	case "performance":
		return "perf"
	default:
		return t
	}
}

func (c commitItem) sectionKey() string {
	switch c.Type {
	case "feat", "fix", "docs", "test", "refactor", "perf", "build", "ci", "style", "chore", "revert":
		return c.Type
	default:
		return "other"
	}
}

func writeSection(out *strings.Builder, title string, items []commitItem, breaking bool) {
	fmt.Fprintf(out, "### %s\n\n", title)
	for _, item := range items {
		fmt.Fprintf(out, "- %s\n", formatBullet(item, breaking))
	}
	out.WriteByte('\n')
}

func formatBullet(item commitItem, breaking bool) string {
	if breaking {
		label := item.Type
		if item.Scope != "" {
			label += "(" + item.Scope + ")"
		}
		label += "!"
		return fmt.Sprintf("`%s`: %s (`%s`)", label, item.Subject, shortHash(item.Hash))
	}

	switch item.sectionKey() {
	case "other":
		if item.Type == "other" {
			return fmt.Sprintf("%s (`%s`)", item.Raw, shortHash(item.Hash))
		}
		label := item.Type
		if item.Scope != "" {
			label += "(" + item.Scope + ")"
		}
		return fmt.Sprintf("`%s`: %s (`%s`)", label, item.Subject, shortHash(item.Hash))
	default:
		if item.Scope != "" {
			return fmt.Sprintf("%s: %s (`%s`)", item.Scope, item.Subject, shortHash(item.Hash))
		}
		return fmt.Sprintf("%s (`%s`)", item.Subject, shortHash(item.Hash))
	}
}

func shortHash(hash string) string {
	hash = strings.TrimSpace(hash)
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
