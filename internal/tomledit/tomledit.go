// Package tomledit changes single keys in a hand-written TOML file without
// re-encoding it, so the comments and layout a person wrote survive.
//
// It understands only what Scriptorium's own files use: top-level keys and
// [table] headers holding simple `key = value` lines. Values are passed and
// written as raw TOML, already quoted by the caller.
package tomledit

import "strings"

// Set gives key the raw TOML value in table, where the empty table is the
// top level. An existing line is replaced in place; a missing key is added
// just under its table header, or above the first table for a top-level key;
// a missing table is appended to the end of the file.
func Set(body, table, key, value string) string {
	lines := strings.Split(body, "\n")
	start, end, found := section(lines, table)
	line := key + " = " + value
	if i, ok := keyLine(lines, start, end, key); ok {
		indent := lines[i][:len(lines[i])-len(strings.TrimLeft(lines[i], " \t"))]
		lines[i] = indent + line
		return strings.Join(lines, "\n")
	}
	if found {
		if table == "" {
			return strings.Join(insert(lines, lastContent(lines, start, end)+1, line), "\n")
		}
		return strings.Join(insert(lines, start, line), "\n")
	}
	trimmed := strings.TrimRight(body, "\n")
	return trimmed + "\n\n[" + table + "]\n" + line + "\n"
}

// Delete removes key from table, leaving everything else as written. A key
// that is not there is not an error.
func Delete(body, table, key string) string {
	lines := strings.Split(body, "\n")
	start, end, _ := section(lines, table)
	if i, ok := keyLine(lines, start, end, key); ok {
		lines = append(lines[:i], lines[i+1:]...)
	}
	return strings.Join(lines, "\n")
}

// section finds the lines belonging to table: from just after its header (or
// the top of the file) up to the next header.
func section(lines []string, table string) (start, end int, found bool) {
	start, found = 0, table == ""
	if !found {
		for i, line := range lines {
			if strings.TrimSpace(line) == "["+table+"]" {
				start, found = i+1, true
				break
			}
		}
		if !found {
			return 0, 0, false
		}
	}
	end = len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "[") {
			end = i
			break
		}
	}
	return start, end, true
}

func keyLine(lines []string, start, end int, key string) (int, bool) {
	for i := start; i < end; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, key+" =") || strings.HasPrefix(trimmed, key+"=") {
			return i, true
		}
	}
	return 0, false
}

// lastContent is the last non-blank line in [start, end), or start-1 when the
// range holds nothing but blank lines.
func lastContent(lines []string, start, end int) int {
	for i := end - 1; i >= start; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return i
		}
	}
	return start - 1
}

func insert(lines []string, at int, line string) []string {
	return append(lines[:at], append([]string{line}, lines[at:]...)...)
}
