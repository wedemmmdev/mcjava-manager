package download

import (
	"crypto/sha1" // #nosec G505 -- SHA1 is used only to verify provider checksums.
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"

	"github.com/wedemmmdev/modink/pkg/modsource"
)

type hashSpec struct {
	algorithm modsource.HashAlgorithm
	expected  string
	hasher    hash.Hash
}

func prepareHashes(values []modsource.Hash) ([]hashSpec, error) {
	seen := make(map[modsource.HashAlgorithm]string, 2)
	specs := make([]hashSpec, 0, 2)

	for _, algorithm := range []modsource.HashAlgorithm{modsource.HashSHA512, modsource.HashSHA1} {
		for _, value := range values {
			normalizedAlgorithm := modsource.HashAlgorithm(strings.ToLower(strings.TrimSpace(string(value.Algorithm))))
			if normalizedAlgorithm != algorithm {
				continue
			}

			expected := strings.ToLower(strings.TrimSpace(value.Value))
			if expected == "" {
				continue
			}
			if previous, ok := seen[algorithm]; ok {
				if previous != expected {
					return nil, fmt.Errorf("%w: valores conflitantes para %s", ErrInvalidChecksum, algorithm)
				}
				continue
			}

			expectedLength := 128
			var hasher hash.Hash = sha512.New()
			if algorithm == modsource.HashSHA1 {
				expectedLength = 40
				hasher = sha1.New() // #nosec G401 -- compatibility checksum from the provider.
			}
			if !validHex(expected, expectedLength) {
				return nil, fmt.Errorf("%w: valor %s malformado", ErrInvalidChecksum, algorithm)
			}

			seen[algorithm] = expected
			specs = append(specs, hashSpec{
				algorithm: algorithm,
				expected:  expected,
				hasher:    hasher,
			})
		}
	}

	if len(specs) == 0 {
		return nil, ErrNoChecksum
	}

	return specs, nil
}

func verifyHashes(specs []hashSpec) (map[modsource.HashAlgorithm]string, error) {
	actual := make(map[modsource.HashAlgorithm]string, len(specs))

	for _, spec := range specs {
		value := hex.EncodeToString(spec.hasher.Sum(nil))
		if !strings.EqualFold(value, spec.expected) {
			return nil, &ChecksumError{
				Algorithm: spec.algorithm,
				Expected:  spec.expected,
				Actual:    value,
			}
		}
		actual[spec.algorithm] = value
	}

	return actual, nil
}

func validHex(value string, length int) bool {
	if len(value) != length {
		return false
	}

	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}

	return true
}
