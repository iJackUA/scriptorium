package library

import (
	"errors"
	"strings"
	"testing"
)

func TestValidBookCodesAreAccepted(t *testing.T) {
	for _, code := range []string{"a", "solaris", "book-2", "book_2", "War1984", "0001"} {
		if err := ValidateBookCode(code); err != nil {
			t.Errorf("ValidateBookCode(%q) = %v, want nil", code, err)
		}
	}
}

// The Book Code names a folder, so what it may not contain is anything that
// would make the folder name something other than what the user typed.
func TestBookCodesThatCannotNameAFolderAreRejected(t *testing.T) {
	for name, code := range map[string]string{
		"empty":            "",
		"blank":            "   ",
		"a space":          "sherlock holmes",
		"a slash":          "holmes/memoirs",
		"a backslash":      `holmes\memoirs`,
		"the current dir":  ".",
		"the parent dir":   "..",
		"a hidden file":    ".holmes",
		"a leading hyphen": "-holmes",
		"a colon":          "holmes:1",
		"a null byte":      "holmes\x00",
		"non-ascii":        "шерлок",
		"far too long":     strings.Repeat("a", maxBookCodeLength+1),
	} {
		err := ValidateBookCode(code)
		if err == nil {
			t.Errorf("%s: ValidateBookCode(%q) was accepted", name, code)
			continue
		}
		if !errors.Is(err, ErrInvalidBookCode) {
			t.Errorf("%s: got %v, want ErrInvalidBookCode", name, err)
		}
	}
}

// The user has to be able to fix the code, so the message has to say what a
// code may hold rather than only that this one is wrong.
func TestTheRejectionSaysWhatACodeMayContain(t *testing.T) {
	err := ValidateBookCode("sherlock holmes")
	if err == nil {
		t.Fatal("accepted a code with a space")
	}
	for _, want := range []string{"letters", "digits", "hyphens", "underscores"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not mention %q", err, want)
		}
	}
}

func TestSeriesCodesAreDerivedFromTheName(t *testing.T) {
	for name, want := range map[string]string{
		"Solaris":                           "solaris",
		"The Adventures of Sherlock Holmes": "the-adventures-of-sherlock-holmes",
		"  spaced  out  ":                   "spaced-out",
		"Hitch-Hiker's Guide":               "hitch-hiker-s-guide",
		"1984":                              "1984",
		"Война и мир":                       fallbackSeriesCode,
		"":                                  fallbackSeriesCode,
		"???":                               fallbackSeriesCode,
	} {
		if got := seriesCodeFor(name); got != want {
			t.Errorf("seriesCodeFor(%q) = %q, want %q", name, got, want)
		}
	}
}

// A derived code is never shown to the user to approve, so a name reused for a
// second Series must not land on the folder the first one already has.
func TestADerivedSeriesCodeStepsAsideForOneAlreadyTaken(t *testing.T) {
	taken := map[string]bool{"solaris": true, "solaris-2": true}

	if got := unusedCode("solaris", taken); got != "solaris-3" {
		t.Errorf("got %q, want %q", got, "solaris-3")
	}
	if got := unusedCode("holmes", taken); got != "holmes" {
		t.Errorf("got %q, want %q", got, "holmes")
	}
}
