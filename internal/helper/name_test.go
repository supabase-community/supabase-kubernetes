package helper

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation"
)

func TestResourceNames(t *testing.T) {
	names := []string{"123", "c-123", "a.b", "a-b", strings.Repeat("a", 253), strings.Repeat("a", 252) + "b"}
	seen := map[string]bool{}
	for _, name := range names {
		value := ResourceName(name, "edge-runtime-main")
		if problems := validation.IsDNS1123Label(value); len(problems) > 0 {
			t.Fatalf("invalid child name %s: %v", value, problems)
		}
		if seen[value] {
			t.Fatalf("name collision: %s", value)
		}
		seen[value] = true
		if ResourceName(name, "edge-runtime-main") != value {
			t.Fatal("name is not stable")
		}
	}
}
