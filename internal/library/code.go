package library

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// maxBookCodeLength keeps a Book Code short enough to be typed and read, which
// is the whole reason it is assigned by hand rather than generated.
const maxBookCodeLength = 64

// fallbackSeriesCode names a Series whose name has nothing a folder name can
// be built out of — a title written entirely in a non-Latin script, say. The
// folder still has to be called something, and it is only ever a folder name;
// the Series' real name is in its config.
const fallbackSeriesCode = "series"

// ErrInvalidBookCode reports a Book Code that cannot name a folder.
var ErrInvalidBookCode = errors.New("that Book Code cannot be used as a folder name")

// ValidateBookCode reports whether code can name the folder a Book is stored
// under.
//
// The rule is an allowlist rather than a list of forbidden characters, because
// the set of characters that mean something to a filesystem, a shell, or a URL
// differs on every platform this runs on, and the Book Code has to name the
// same folder on all of them. What survives is what the user would have chosen
// anyway: the code is theirs to type and theirs to find on disk.
func ValidateBookCode(code string) error {
	switch {
	case strings.TrimSpace(code) == "":
		return fmt.Errorf("%w: a Book Code cannot be empty", ErrInvalidBookCode)
	case len(code) > maxBookCodeLength:
		return fmt.Errorf("%w: %q is longer than %d characters", ErrInvalidBookCode, code, maxBookCodeLength)
	}
	for _, r := range code {
		if !isCodeRune(r) {
			return fmt.Errorf("%w: %q may contain only letters, digits, hyphens and underscores", ErrInvalidBookCode, code)
		}
	}
	// A leading hyphen reads as a flag to every command-line tool the user
	// might point at the folder, and a leading dot hides it from them.
	if first := code[0]; !isAlphanumeric(rune(first)) {
		return fmt.Errorf("%w: %q must start with a letter or a digit", ErrInvalidBookCode, code)
	}
	return nil
}

func isCodeRune(r rune) bool { return isAlphanumeric(r) || r == '-' || r == '_' }

func isAlphanumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// seriesCodeFor derives the folder name for a Series from its name.
//
// The Series code is derived rather than typed, because the ticket asks the
// user for a Book Code and nothing else: being asked to name a folder twice —
// once for the Series and once for the Book inside it — is the ceremony a
// standalone Book is supposed to avoid. A derived name can collide, which is
// what unusedCode is for.
func seriesCodeFor(name string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case isAlphanumeric(r):
			out.WriteRune(r)
		// Runs of anything else collapse to a single hyphen, so "  spaced  out  "
		// does not become "-spaced--out-".
		case !strings.HasSuffix(out.String(), "-"):
			out.WriteByte('-')
		}
	}
	code := strings.Trim(out.String(), "-")
	if code == "" {
		return fallbackSeriesCode
	}
	if len(code) > maxBookCodeLength {
		code = strings.Trim(code[:maxBookCodeLength], "-")
	}
	return code
}

// unusedCode returns code, or code with a number appended, whichever is not in
// taken.
func unusedCode(code string, taken map[string]bool) string {
	if !taken[code] {
		return code
	}
	for n := 2; ; n++ {
		candidate := code + "-" + strconv.Itoa(n)
		if !taken[candidate] {
			return candidate
		}
	}
}
