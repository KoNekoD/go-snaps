package snaps

import (
	"github.com/KoNekoD/go-snaps/internal/test"
	"testing"
)

func TestStringUtils(t *testing.T) {
	t.Run("splitNewlines", func(t *testing.T) {
		for _, v := range []struct {
			input    string
			expected []string
		}{
			{"foo", []string{"foo\n"}},
			{"foo\nbar", []string{"foo\n", "bar\n"}},
			{"foo\nbar\n", []string{"foo\n", "bar\n", "\n"}},
			{`abc
			efg
			hello \n world`, []string{"abc\n", "\t\t\tefg\n", "\t\t\thello \\n world\n"}},
		} {
			v := v
			t.Run(v.input, func(t *testing.T) {
				t.Parallel()
				test.Equal(t, v.expected, splitNewlines(v.input))
			})
		}
	})

	t.Run("isSingleLine", func(t *testing.T) {
		test.True(t, isSingleline("hello world"))
		test.True(t, isSingleline("hello world\n"))
		test.False(t, isSingleline(`hello 
		 world
		 `))
		test.False(t, isSingleline("hello \n world\n"))
		test.False(t, isSingleline("hello \n world"))
	})
}

func TestDiff(t *testing.T) {
	t.Run("should build diff report consistently", func(t *testing.T) {
		MatchSnapshot(t, buildDiffReport(10000, 20, "mock-diff", "snap/path", 10))
		MatchSnapshot(t, buildDiffReport(20, 10000, "mock-diff", "snap/path", 20))
	})

	t.Run("should not print diff report if no diffs", func(t *testing.T) {
		test.Equal(t, "", buildDiffReport(0, 0, "", "", -1))
	})

	t.Run("should not print snapshot line if not provided", func(t *testing.T) {
		MatchSnapshot(t, buildDiffReport(10, 2, "there is a diff here", "", -1))
	})
}
