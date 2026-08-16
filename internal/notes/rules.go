package notes

import (
	crand "crypto/rand"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"
)

var segmentPattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)
var titlePattern = regexp.MustCompile(`^[A-Za-z0-9 -]+$`)

// ValidSegment reports whether s contains only letters, numbers, and hyphens.
func ValidSegment(s string) bool {
	return s != "" && segmentPattern.MatchString(s)
}

// ValidTitle reports whether s contains only letters, numbers, spaces, and
// hyphens.
func ValidTitle(s string) bool {
	return s != "" && titlePattern.MatchString(s)
}

// NormalizeTitle lowercases a title and replaces runs of whitespace with a
// single hyphen.
func NormalizeTitle(title string) string {
	return strings.Join(strings.Fields(strings.ToLower(title)), "-")
}

// ParentOf returns the parent directory of a note's relative path, or "" for a
// note at the root.
func ParentOf(dir string) string {
	if idx := strings.LastIndex(dir, "/"); idx >= 0 {
		return dir[:idx]
	}
	return ""
}

// NormalizePath splits a "/"-separated path, drops empty segments, lowercases
// each segment, and rejoins them.
func NormalizePath(input string) string {
	var segments []string
	for _, segment := range strings.Split(input, "/") {
		segment = strings.ToLower(strings.TrimSpace(segment))
		if segment == "" {
			continue
		}
		segments = append(segments, segment)
	}
	return strings.Join(segments, "/")
}

// NewID returns a random note identifier.
func NewID() string {
	bytes := make([]byte, 16)
	if _, err := crand.Read(bytes); err != nil {
		panic(fmt.Sprintf("generate identifier: %v", err))
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

func ParseFilter(value string) Filter {
	filter := Filter{}
	var content []string
	for _, token := range strings.Fields(value) {
		parts := strings.SplitN(token, ":", 2)
		if len(parts) != 2 {
			content = append(content, token)
			continue
		}
		switch parts[0] {
		case "tag":
			filter.Tag = parts[1]
		case "after":
			if date, err := time.Parse("2006-01-02", parts[1]); err == nil {
				filter.From = &date
			}
		case "before":
			if date, err := time.Parse("2006-01-02", parts[1]); err == nil {
				filter.Through = &date
			}
		default:
			content = append(content, token)
		}
	}
	filter.Query = strings.Join(content, " ")
	return filter
}

func GeneratedTitle() string {
	adjectives := []string{"amber", "brisk", "calm", "clever", "cosmic", "gentle", "quiet", "tiny"}
	animals := []string{"badger", "cat", "fox", "otter", "panda", "rabbit", "shiba", "tiger"}
	return adjectives[rand.IntN(len(adjectives))] + "-" + animals[rand.IntN(len(animals))]
}

func NormalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	result := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.ToLower(strings.TrimSpace(raw))
		if tag == "" {
			continue
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return result
}
