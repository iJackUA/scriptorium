package tomledit

import "testing"

func TestSetReplacesAKeyInPlaceKeepingComments(t *testing.T) {
	body := "# note\n[translation]\n# the strong one\nmodel = \"old\"\neffort = \"low\"\n"
	got := Set(body, "translation", "model", `"new"`)
	want := "# note\n[translation]\n# the strong one\nmodel = \"new\"\neffort = \"low\"\n"
	if got != want {
		t.Errorf("Set =\n%s\nwant\n%s", got, want)
	}
}

func TestSetOnlyTouchesItsOwnTable(t *testing.T) {
	body := "[mechanical]\nmodel = \"cheap\"\n\n[translation]\nmodel = \"strong\"\n"
	got := Set(body, "translation", "model", `"stronger"`)
	want := "[mechanical]\nmodel = \"cheap\"\n\n[translation]\nmodel = \"stronger\"\n"
	if got != want {
		t.Errorf("Set =\n%s\nwant\n%s", got, want)
	}
}

func TestSetAddsAMissingKeyUnderItsTableHeader(t *testing.T) {
	body := "[translation]\nmodel = \"strong\"\n\n[other]\nx = 1\n"
	got := Set(body, "translation", "effort", `"high"`)
	want := "[translation]\neffort = \"high\"\nmodel = \"strong\"\n\n[other]\nx = 1\n"
	if got != want {
		t.Errorf("Set =\n%s\nwant\n%s", got, want)
	}
}

func TestSetAppendsAMissingTable(t *testing.T) {
	body := "title = \"Solaris\"\n# [translation]\n# model = \"x\"\n"
	got := Set(body, "translation", "model", `"strong"`)
	want := "title = \"Solaris\"\n# [translation]\n# model = \"x\"\n\n[translation]\nmodel = \"strong\"\n"
	if got != want {
		t.Errorf("Set =\n%s\nwant\n%s", got, want)
	}
}

func TestSetAtTopLevelStaysAboveTheFirstTable(t *testing.T) {
	body := "title = \"Solaris\"\n\n[models]\ntranslation = \"x\"\n"
	got := Set(body, "", "agent", `"claude"`)
	want := "title = \"Solaris\"\nagent = \"claude\"\n\n[models]\ntranslation = \"x\"\n"
	if got != want {
		t.Errorf("Set =\n%s\nwant\n%s", got, want)
	}
}

func TestDeleteRemovesOnlyThatKey(t *testing.T) {
	body := "agent = \"claude\"\n\n[models]\nmechanical = \"a\"\ntranslation = \"b\"\n"
	got := Delete(Delete(body, "models", "translation"), "", "agent")
	want := "\n[models]\nmechanical = \"a\"\n"
	if got != want {
		t.Errorf("Delete =\n%q\nwant\n%q", got, want)
	}
	if again := Delete(got, "models", "absent"); again != got {
		t.Errorf("deleting an absent key changed the file:\n%s", again)
	}
}
