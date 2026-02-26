/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package algorithms

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"strings"

	"github.com/wso2/identity-customer-data-service/internal/system/constants"
)

// LSHBandHashes generates LSH blocking key strings for a normalized string.
func LSHBandHashes(normalized string) []string {
	if len(normalized) < constants.LSHMinLength {
		return nil
	}

	bigrams := CharacterBigrams(normalized)
	if len(bigrams) < 2 {
		return nil
	}

	sig := MinHashSignature(bigrams)
	keys := make([]string, 0, constants.LSHBands)

	for band := 0; band < constants.LSHBands; band++ {
		start := band * constants.LSHRows
		end := start + constants.LSHRows
		h := fnv.New64a()
		buf := make([]byte, 8)
		for i := start; i < end; i++ {
			binary.LittleEndian.PutUint64(buf, sig[i])
			_, _ = h.Write(buf)
		}

		keys = append(keys, fmt.Sprintf("lsh:%x", h.Sum64()))
	}

	return keys
}

// CharacterBigrams extracts all unique character bigrams from a string.
func CharacterBigrams(s string) []string {
	runes := []rune(strings.ToLower(s))
	if len(runes) < 2 {
		return nil
	}

	seen := make(map[string]bool)
	bigrams := make([]string, 0, len(runes)-1)
	for i := 0; i < len(runes)-1; i++ {
		bg := string(runes[i : i+2])
		if !seen[bg] {
			seen[bg] = true
			bigrams = append(bigrams, bg)
		}
	}

	return bigrams
}

// MinHashSignature computes a min-hash signature for a set of bigrams.
func MinHashSignature(bigrams []string) []uint64 {
	sig := make([]uint64, constants.LSHSignatureSize)
	for i := range sig {
		sig[i] = math.MaxUint64
	}

	for _, bg := range bigrams {
		h := fnv.New64a()
		_, _ = h.Write([]byte(bg))
		baseHash := h.Sum64()

		for i := 0; i < constants.LSHSignatureSize; i++ {
			a := uint64(i*2654435761 + 1)
			b := uint64(i*1103515245 + 12345)
			hVal := a*baseHash + b

			if hVal < sig[i] {
				sig[i] = hVal
			}
		}
	}

	return sig
}
