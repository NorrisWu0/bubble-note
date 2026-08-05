package notes

import (
	"math/rand/v2"
	"strings"
	"time"
)

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
