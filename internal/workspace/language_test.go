package workspace

import "testing"

func TestCatalogResolvesCanonicalTagsToEnglishNames(t *testing.T) {
	for tag, want := range map[string]string{"en": "English", "uk": "Ukrainian", "de": "German"} {
		language, ok := LanguageFor(tag)
		if !ok || language.Name != want {
			t.Errorf("LanguageFor(%q) = %+v, %v; want %q", tag, language, ok, want)
		}
	}
}

func TestCatalogRejectsUnknownAndNonCanonicalTags(t *testing.T) {
	for _, tag := range []string{"English", "EN", "zz", "en-GB"} {
		if _, ok := LanguageFor(tag); ok {
			t.Errorf("LanguageFor(%q) accepted an invalid tag", tag)
		}
	}
}
