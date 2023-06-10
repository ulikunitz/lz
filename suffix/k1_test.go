// SPDX-FileCopyrightText: © 2021 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package suffix

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"strings"
	"testing"
)

func TestSort(t *testing.T) {

	tests := []string{
		"abbaabbaabbaabba",
		"ababababababababac",
		"cdcdcdcdccdd$",
		"banana",
		"christmas",
		"cba",
		"The brown fox jumps over the lazy dog.",
		"<mediawiki xmlns=\"http://www.mediawik",
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			cfg := config{
				sizeThreshold: 7,
				t:             t,
			}
			text := []byte(tc)
			sa := make([]int32, len(text))
			cfg.sort(text, sa)
			if err := verifySuffixArray(text, sa); err != nil {
				t.Fatal(err)
			}
		})
	}
}

const testFile = "../testdata/enwik7"

func getData(file string) (data []byte, err error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, 1000000))
}

func shorter(s []byte) string {
	if len(s) > 16 {
		return fmt.Sprintf("%s...", s[:16])
	}
	return string(s)
}


func TestEnwik6(t *testing.T) {
	cfg := config{
		sizeThreshold: 8,
		t:             t,
	}
	data, err := getData(testFile)
	if err != nil {
		t.Fatalf("getData(%q) error %s", testFile, err)
	}
	for n := len(data); n <= len(data); n++ {
		p := make([]byte, n)
		copy(p, data)
		sa := make([]int32, len(p))
		cfg.sort(p, sa)
		if err := verifySuffixArray(p, sa); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEnwik6Debug(t *testing.T) {
	cfg := config{
		sizeThreshold: 8,
		t:             t,
	}
	data, err := getData(testFile)
	if err != nil {
		t.Fatalf("getData(%q) error %s", testFile, err)
	}
	// n := 662
	p := data[272:662]
	sa := make([]int32, len(p))
	cfg.sort(p, sa)
	if err := verifySuffixArray(p, sa); err != nil {
		t.Fatal(err)
	}
}

func TestFindIndexes(t *testing.T) {
	a := []int{0, 5, 7, 10, 14, 16, 18, 20, 29, 31, 35, 42, 44, 46, 48, 51,
		55, 58, 60, 66, 70, 74, 76, 85, 87, 89, 91, 93, 96, 98, 102,
		105, 110, 115, 117, 119, 121, 123, 131, 133, 140, 142, 144,
		147, 150, 152, 163, 165, 167, 172, 182, 184, 186, 190, 192,
		194, 198, 202, 204, 208, 210, 212, 216, 226, 228, 230, 234,
		236, 238, 242, 246, 249, 251, 254, 256, 258, 262, 272, 274,
		276, 280, 282, 284, 287, 291, 300, 302, 304, 308, 310, 312,
		315, 320, 324, 326, 328, 332, 342, 344, 346, 350, 352, 354,
		357, 361, 363, 366, 368, 370, 374, 384, 386, 388}
	for i, k := range a {
		if k == 384 {
			t.Logf("%d -> %d", i, k)
		}
		if k == 163 {
			t.Logf("%d -> %d", i, k)
		}
	}

	isa := []int{51, 103, 106, 71, 21, 107, 74, 40, 23, 56, 99, 73, 12, 102,
		105, 69, 111, 22, 101, 104, 37, 14, 54, 25, 76, 80, 58, 110,
		34, 72, 100, 0, 108, 16, 75, 79, 57, 109, 24, 55, 112, 11, 93,
		77, 15, 53, 31, 50, 92, 82, 27, 42, 84, 60, 2, 95, 10, 33, 70,
		17, 46, 88, 64, 26, 41, 83, 59, 1, 94, 9, 35, 68, 38, 18, 47,
		89, 65, 28, 43, 85, 61, 3, 96, 6, 13, 29, 44, 86, 62, 4, 97, 7,
		39, 19, 48, 90, 66, 30, 45, 87, 63, 5, 98, 8, 36, 78, 20, 49,
		91, 67, 32, 52, 81}
	for _, i := range []int{46, 110} {
		t.Logf("isa[%d]: %d", i, isa[i])
	}
}

func BenchmarkSizeThreshold(b *testing.B) {
	data, err := getData(testFile)
	if err != nil {
		b.Fatalf("getData(%q) error %s", testFile, err)
	}

	var cfg config
	for th := 4; th < 25; th++ {
		th := th
		b.Run(fmt.Sprintf("%d", th), func(b *testing.B) {
			p := make([]byte, len(data))
			copy(p, data)
			sa := make([]int32, len(p))
			b.SetBytes(int64(len(data)))
			cfg.sizeThreshold = th
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cfg.sort(p, sa)
			}
		})
	}
}

func TestAbsNeg(t *testing.T) {
	tests := []struct{ x, y int32 }{
		{0, 0},
		{1, 1},
		{^1, 1},
		{1<<31 - 1, 1<<31 - 1},
		{^(1<<31 - 1), 1<<31 - 1},
	}
	for _, tc := range tests {
		y := absNeg(tc.x)
		if y != tc.y {
			t.Fatalf("absNeg(%d) returned %d; want %d",
				tc.x, y, tc.y)
		}
	}
}

type sortError struct {
	data    []byte
	sa      []int32
	i, j    int
	greater bool
}

func (err *sortError) Error() string {
	u := err.data[err.sa[err.i]:]
	v := err.data[err.sa[err.j]:]
	var rel string
	if err.greater {
		rel = ">"
	} else {
		rel = ">="
	}
	return fmt.Sprintf(
		"data[sa[%d]=%d:]=%s %s data[sa[%d]=%d:]=%s",
		err.i, err.sa[err.i], shorter(u), rel,
		err.j, err.sa[err.j], shorter(v))
}

func verifyPermutation(a []int32) error {
	b := make([]int32, len(a))
	for i := range b {
		b[i] = -1
	}
	for i, j := range a {
		if j < 0 {
			return fmt.Errorf("a[%d]=%d is negative;"+
				" want non-negative value", i, j)
		}
		if b[j] >= 0 {
			return fmt.Errorf("a[%d]=%d conflicts with a[%d]=%d",
				i, j, b[j], j)
		}
		b[j] = int32(i)
	}
	return nil
}

func verifySuffixArray(t []byte, sa []int32) error {
	if len(t) != len(sa) {
		return fmt.Errorf("len(t)=%d != len(sa)=%d", len(t), len(sa))
	}
	if len(sa) == 0 {
		return nil
	}
	// Check that the suffix array is actual a permutation.
	if err := verifyPermutation(sa); err != nil {
		return err
	}
	var (
		u, v []byte
	)
	// For data with a length less or equal 1 MiB we are doing a full
	// comparison of all suffixes.
	if len(sa) <= 1<<20 {
		v = t[sa[0]:]
		for i, k := range sa[1:] {
			u, v = v, t[k:]
			if bytes.Compare(u, v) >= 0 {
				return &sortError{sa: sa, data: t, i: i,
					j: i + 1}
			}
		}
		return nil
	}
	// Otherwise we are checking the first 10 kByte of each suffix.
	const (
		maxLen  = 10 << 10
		samples = 10000
	)
	v = t[sa[0]:]
	if len(v) > maxLen {
		v = v[:maxLen]
	}
	for i, k := range sa[1:] {
		u, v = v, t[k:]
		if len(v) > maxLen {
			v = v[:maxLen]
		}
		if bytes.Compare(u, v) > 0 {
			return &sortError{sa: sa, data: t, i: i, j: i + 1,
				greater: true}
		}
	}
	// For completeness we are taking samples with a comparison of the
	// full suffixes.
	for k := 0; k < samples; k++ {
		i := rand.Intn(len(sa))
		u = t[sa[i]:]
		v = t[sa[i+1]:]
		if bytes.Compare(u, v) >= 0 {
			return &sortError{sa: sa, data: t, i: i, j: i + 1}
		}
	}
	return nil
}

func TestCorpora(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	const corporaDir = "../../testdata/corpora"
	corporaFS := os.DirFS(corporaDir)
	filenames, err := fs.Glob(corporaFS, "*/*")
	if err != nil {
		t.Fatalf("fs.Glob(corporaFS, %q) error %s",
			"*/*", err)
	}
	for _, fn := range filenames {
		filename := fn
		if strings.HasSuffix(filename, "README.md") {
			continue
		}
		ok := t.Run(filename, func(t *testing.T) {
			t.Parallel()
			data, err := fs.ReadFile(corporaFS, filename)
			if err != nil {
				t.Fatalf("fs.ReadFile(corporaFS, %q) error %s",
					filename, err)
			}
			p := make([]byte, len(data))
			copy(p, data)
			sa := make([]int32, len(data))
			Sort(p, sa)
			if err = verifySuffixArray(data, sa); err != nil {
				t.Error(err)
			}
		})
		if !ok {
			t.Fatal()
		}
	}
}

func TestVerifyPermutation(t *testing.T) {
	tests := [][]int32{
		{1, 1, 1},
		{-1, 2, 3},
	}
	for _, tc := range tests {
		if err := verifyPermutation(tc); err == nil {
			t.Fatalf("verifyPermutation(%d) returned no error", tc)
		}
	}
}

func TestVerifySuffixArray(t *testing.T) {
	tests := []struct {
		t  []byte
		sa []int32
	}{
		{t: []byte("abba"), sa: []int32{3, 2, 0, 1}},
	}
	for _, tc := range tests {
		if err := verifySuffixArray(tc.t, tc.sa); err == nil {
			t.Fatalf("verifySuffixArray(%q, %d) no error", tc.t,
				tc.sa)
		}
	}
}
