package notes

import "testing"

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{" Work ", "work", "", "Personal"})
	want := []string{"work", "personal"}
	if len(got) != len(want) {
		t.Fatalf("tags = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tags = %#v, want %#v", got, want)
		}
	}
}

func TestParseFilter(t *testing.T) {
	filter := ParseFilter("jasmine tag:shopping after:2026-01-01")
	if filter.Query != "jasmine" || filter.Tag != "shopping" || filter.From == nil {
		t.Fatalf("filter = %+v, want content, tag, and date", filter)
	}
}

func TestValidSegment(t *testing.T) {
	valid := []string{"abc", "ABC", "a-b", "123", "a1-b2"}
	for _, s := range valid {
		if !ValidSegment(s) {
			t.Fatalf("ValidSegment(%q) = false, want true", s)
		}
	}
	invalid := []string{"", "a b", "a_b", "a.b", "a/b", "héllo"}
	for _, s := range invalid {
		if ValidSegment(s) {
			t.Fatalf("ValidSegment(%q) = true, want false", s)
		}
	}
}

func TestNormalizePath(t *testing.T) {
	tests := map[string]string{
		"/Journal/Bubble-Note/": "journal/bubble-note",
		"a//b":                  "a/b",
		"  Journal / Notes ":    "journal/notes",
		"":                      "",
		"///":                   "",
	}
	for input, want := range tests {
		if got := NormalizePath(input); got != want {
			t.Fatalf("NormalizePath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestValidTitle(t *testing.T) {
	valid := []string{"abc", "My Note", "a-b c", "123"}
	for _, s := range valid {
		if !ValidTitle(s) {
			t.Fatalf("ValidTitle(%q) = false, want true", s)
		}
	}
	invalid := []string{"", "a_b", "a.b", "a/b", "héllo"}
	for _, s := range invalid {
		if ValidTitle(s) {
			t.Fatalf("ValidTitle(%q) = true, want false", s)
		}
	}
}

func TestNormalizeTitle(t *testing.T) {
	tests := map[string]string{
		"My Note":           "my-note",
		"  Hello   World  ": "hello-world",
		"Already-Done":      "already-done",
	}
	for input, want := range tests {
		if got := NormalizeTitle(input); got != want {
			t.Fatalf("NormalizeTitle(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParentOf(t *testing.T) {
	tests := map[string]string{
		"journal/bubble-note/nested-notes": "journal/bubble-note",
		"docs/github":                      "docs",
		"note":                             "",
	}
	for dir, want := range tests {
		if got := ParentOf(dir); got != want {
			t.Fatalf("ParentOf(%q) = %q, want %q", dir, got, want)
		}
	}
}
