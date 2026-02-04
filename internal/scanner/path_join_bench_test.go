//go:build windows

package scanner

import (
	"path/filepath"
	"strings"
	"testing"
)

// BenchmarkPathJoin compares the performance of different path joining strategies
// to validate the optimization of replacing filepath.Join with direct string concatenation.
//
// This benchmark measures three approaches:
// 1. filepath.Join - The original approach (slow, validates and cleans paths)
// 2. String concatenation - Simple approach (faster, but multiple allocations)
// 3. strings.Builder - Optimized approach (fastest, single allocation per path)
//
// Expected results:
// - filepath.Join: ~200 ns/op, 2-3 allocs/op
// - string concat: ~50 ns/op, 1-2 allocs/op
// - strings.Builder: ~40 ns/op, 1 alloc/op

// BenchmarkPathJoin_FilepathJoin benchmarks the original filepath.Join approach
func BenchmarkPathJoin_FilepathJoin(b *testing.B) {
	dirPath := `C:\some\very\long\directory\path\with\many\subdirectories`
	filename := "somefile_with_a_reasonably_long_name.txt"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = filepath.Join(dirPath, filename)
	}
}

// BenchmarkPathJoin_StringConcat benchmarks simple string concatenation
func BenchmarkPathJoin_StringConcat(b *testing.B) {
	dirPath := `C:\some\very\long\directory\path\with\many\subdirectories`
	filename := "somefile_with_a_reasonably_long_name.txt"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dirPath + string(filepath.Separator) + filename
	}
}

// BenchmarkPathJoin_StringBuilder benchmarks the optimized strings.Builder approach
func BenchmarkPathJoin_StringBuilder(b *testing.B) {
	dirPath := `C:\some\very\long\directory\path\with\many\subdirectories`
	filename := "somefile_with_a_reasonably_long_name.txt"

	var builder strings.Builder
	builder.Grow(len(dirPath) + 1 + len(filename))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder.Reset()
		builder.WriteString(dirPath)
		builder.WriteByte(filepath.Separator)
		builder.WriteString(filename)
		_ = builder.String()
	}
}

// BenchmarkPathJoin_StringBuilderRealistic simulates the real-world scenario
// where the builder is reused across multiple paths with varying filename lengths
func BenchmarkPathJoin_StringBuilderRealistic(b *testing.B) {
	dirPath := `C:\some\very\long\directory\path\with\many\subdirectories`
	filenames := []string{
		"short.txt",
		"medium_length_filename.txt",
		"very_long_filename_that_might_appear_in_real_world_scenarios.txt",
		"a.txt",
		"another_reasonably_long_name_for_testing.doc",
	}

	var builder strings.Builder
	builder.Grow(len(dirPath) + 1 + 256) // Pre-allocate for average case

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filename := filenames[i%len(filenames)]
		builder.Reset()
		builder.WriteString(dirPath)
		builder.WriteByte(filepath.Separator)
		builder.WriteString(filename)
		_ = builder.String()
	}
}

// BenchmarkPathJoin_WithUNCPath benchmarks UNC paths (\\server\share\path)
func BenchmarkPathJoin_WithUNCPath(b *testing.B) {
	dirPath := `\\server\share\some\very\long\directory\path`
	filename := "somefile_with_a_reasonably_long_name.txt"

	b.Run("filepath.Join", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = filepath.Join(dirPath, filename)
		}
	})

	b.Run("strings.Builder", func(b *testing.B) {
		var builder strings.Builder
		builder.Grow(len(dirPath) + 1 + len(filename))

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			builder.Reset()
			builder.WriteString(dirPath)
			builder.WriteByte(filepath.Separator)
			builder.WriteString(filename)
			_ = builder.String()
		}
	})
}

// BenchmarkPathJoin_ShortPaths benchmarks with short paths (common case)
func BenchmarkPathJoin_ShortPaths(b *testing.B) {
	dirPath := `C:\temp`
	filename := "file.txt"

	b.Run("filepath.Join", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = filepath.Join(dirPath, filename)
		}
	})

	b.Run("strings.Builder", func(b *testing.B) {
		var builder strings.Builder
		builder.Grow(len(dirPath) + 1 + len(filename))

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			builder.Reset()
			builder.WriteString(dirPath)
			builder.WriteByte(filepath.Separator)
			builder.WriteString(filename)
			_ = builder.String()
		}
	})
}

// BenchmarkPathJoin_LongPaths benchmarks with very long paths (edge case)
func BenchmarkPathJoin_LongPaths(b *testing.B) {
	dirPath := `C:\` + strings.Repeat("very_long_subdirectory_name\", 20)
	filename := strings.Repeat("very_long_filename_", 10) + ".txt"

	b.Run("filepath.Join", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = filepath.Join(dirPath, filename)
		}
	})

	b.Run("strings.Builder", func(b *testing.B) {
		var builder strings.Builder
		builder.Grow(len(dirPath) + 1 + len(filename))

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			builder.Reset()
			builder.WriteString(dirPath)
			builder.WriteByte(filepath.Separator)
			builder.WriteString(filename)
			_ = builder.String()
		}
	})
}
