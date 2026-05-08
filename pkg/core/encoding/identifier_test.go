package encoding

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRandomString(t *testing.T) {
	for i := 0; i < 1000; i++ {
		assert.Len(t, NewRandomString(i), i)
	}
}

func TestNewRandomIdentifier(t *testing.T) {
	id := NewRandomIdentifier("pre")
	assert.Regexp(t, regexp.MustCompile(`^pre_[1-9A-HJ-NP-Za-km-z]+$`), id)
}

func TestRandomIdentifierWithLength(t *testing.T) {
	id := NewRandomIdentifierWithLength("pre", 8)
	assert.Regexp(t, regexp.MustCompile(`^pre_[1-9A-HJ-NP-Za-km-z]{10,11}$`), id)
}

func runBenchmark(b *testing.B, fn func()) {
	for n := 0; n < b.N; n++ {
		fn()
	}
}

func BenchmarkNewRandomString8(b *testing.B) {
	runBenchmark(b, func() { NewRandomString(8) })
}

func BenchmarkNewRandomString16(b *testing.B) {
	runBenchmark(b, func() { NewRandomString(16) })
}

func BenchmarkNewRandomString32(b *testing.B) {
	runBenchmark(b, func() { NewRandomString(32) })
}

func BenchmarkNewRandomString64(b *testing.B) {
	runBenchmark(b, func() { NewRandomString(64) })
}

func BenchmarkNewRandomString128(b *testing.B) {
	runBenchmark(b, func() { NewRandomString(128) })
}

func BenchmarkNewRandomString256(b *testing.B) {
	runBenchmark(b, func() { NewRandomString(256) })
}

func BenchmarkNewRandomString512(b *testing.B) {
	runBenchmark(b, func() { NewRandomString(512) })
}

func BenchmarkNewRandomString1024(b *testing.B) {
	runBenchmark(b, func() { NewRandomString(1024) })
}

func BenchmarkNewRandomIdentifierWithLength8(b *testing.B) {
	runBenchmark(b, func() { NewRandomIdentifierWithLength("pre", 8) })
}
func BenchmarkNewRandomIdentifierWithLength16(b *testing.B) {
	runBenchmark(b, func() { NewRandomIdentifierWithLength("pre", 16) })
}
func BenchmarkNewRandomIdentifierWithLength32(b *testing.B) {
	runBenchmark(b, func() { NewRandomIdentifierWithLength("pre", 32) })
}
func BenchmarkNewRandomIdentifierWithLength64(b *testing.B) {
	runBenchmark(b, func() { NewRandomIdentifierWithLength("pre", 64) })
}
func BenchmarkNewRandomIdentifierWithLength128(b *testing.B) {
	runBenchmark(b, func() { NewRandomIdentifierWithLength("pre", 128) })
}
func BenchmarkNewRandomIdentifierWithLength256(b *testing.B) {
	runBenchmark(b, func() { NewRandomIdentifierWithLength("pre", 256) })
}
func BenchmarkNewRandomIdentifierWithLength512(b *testing.B) {
	runBenchmark(b, func() { NewRandomIdentifierWithLength("pre", 512) })
}
func BenchmarkNewRandomIdentifierWithLength1024(b *testing.B) {
	runBenchmark(b, func() { NewRandomIdentifierWithLength("pre", 1024) })
}
