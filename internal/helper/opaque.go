/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package helper

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

const opaqueKeyProjectRef = "supabase-self-hosted"

// GenerateOpaqueKey generates an opaque API key with the given prefix.
// Format: <prefix><22-char-random>_<8-char-checksum>
// Checksum: SHA-256 of "supabase-self-hosted|<prefix><random>" truncated to 8 base64url chars.
func GenerateOpaqueKey(prefix string) (string, error) {
	randomBytes := make([]byte, 17)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generating random bytes for opaque key: %w", err)
	}
	random := base64urlEncode(randomBytes)[:22]
	intermediate := prefix + random

	hash := sha256.Sum256([]byte(opaqueKeyProjectRef + "|" + intermediate))
	checksum := base64urlEncode(hash[:])[:8]

	return intermediate + "_" + checksum, nil
}
