package github

import (
	"strings"
)

// parseLinkNext extracts the URL for rel="next" from a GitHub Link header.
func parseLinkNext(linkHeader string) (string, bool) {
	if linkHeader == "" {
		return "", false
	}
	for _, part := range strings.Split(linkHeader, ",") {
		part = strings.TrimSpace(part)
		segments := strings.Split(part, ";")
		if len(segments) < 2 {
			continue
		}
		urlPart := strings.TrimSpace(segments[0])
		if !strings.HasPrefix(urlPart, "<") || !strings.HasSuffix(urlPart, ">") {
			continue
		}
		rel := strings.TrimSpace(segments[1])
		if rel == `rel="next"` || rel == "rel=next" {
			return strings.Trim(urlPart, "<>"), true
		}
	}
	return "", false
}
