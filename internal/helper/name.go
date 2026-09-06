package helper

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// ResourceName bounds child names and label values without conflating dotted names.
func ResourceName(name, suffix string) string {
	original := name + "-" + suffix
	normalized := strings.ReplaceAll(original, ".", "-")
	if normalized[0] >= '0' && normalized[0] <= '9' {
		normalized = "c-" + normalized
	}
	if len(normalized) <= 63 && normalized == original {
		return normalized
	}
	prefix := normalized
	if len(prefix) > 46 {
		prefix = prefix[:46]
	}
	digest := sha256.Sum256([]byte(original))
	return fmt.Sprintf("%s-%x", strings.TrimRight(prefix, "-"), digest[:8])
}
