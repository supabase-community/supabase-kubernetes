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
	"encoding/hex"
	"fmt"
	"math/big"
)

const alphanumericCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateRandomHex generates a cryptographically secure random hex string.
// The numBytes parameter specifies the number of random bytes; the returned
// string will be 2*numBytes characters long.
func GenerateRandomHex(numBytes int) (string, error) {
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateRandomAlphanumeric generates a cryptographically secure random
// alphanumeric string of the specified length.
func GenerateRandomAlphanumeric(length int) (string, error) {
	result := make([]byte, length)
	charsetLen := big.NewInt(int64(len(alphanumericCharset)))
	for i := range result {
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", fmt.Errorf("generating random index: %w", err)
		}
		result[i] = alphanumericCharset[idx.Int64()]
	}
	return string(result), nil
}
