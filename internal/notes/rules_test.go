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
