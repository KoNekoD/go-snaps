package snaps

import (
	"github.com/gkampitakis/go-snaps/snaps/matchers"
)

// MatchJSON verifies the input matches the most recent snap file.
// Input can be a valid json string or []byte or whatever value can be passed
// successfully on `json.Marshal`.
//
//	MatchJSON(t, `{"user":"mock-user","age":10,"email":"mock@email.com"}`)
//	MatchJSON(t, []byte(`{"user":"mock-user","age":10,"email":"mock@email.com"}`))
//	MatchJSON(t, User{10, "mock-email"})
//
// MatchJSON also supports passing matchers as a third argument. Those matchers can act either as
// validators or placeholders for data that might change on each invocation e.g. dates.
//
//	MatchJSON(t, User{created: time.Now(), email: "mock-email"}, match.Any("created"))
func MatchJSON(t TestingT, input any, matchers ...matchers.JsonMatcher) {
	t.Helper()

	defaultSnap.withTesting(t).matchJson(input, matchers...)
}

// MatchSnapshot verifies the values match the most recent snap file
// You can pass multiple values
//
//	MatchSnapshot(t, 10, "hello world")
//
// or call MatchSnapshot multiples times inside a test
//
//	MatchSnapshot(t, 10)
//	MatchSnapshot(t, "hello world")
//
// The difference is the latter will create multiple entries.
func MatchSnapshot(t TestingT, values ...any) {
	t.Helper()

	defaultSnap.withTesting(t).matchSnapshot(values...)
}

// MatchStandaloneSnapshot verifies the value matches the most recent snap file
//
//	MatchStandaloneSnapshot(t, "Hello World")
//
// MatchStandaloneSnapshot creates one snapshot file per call.
//
// You can call MatchStandaloneSnapshot multiple times inside a test.
// It will create multiple snapshot files at `__snapshots__` folder by default.
func MatchStandaloneSnapshot(t TestingT, value any) {
	t.Helper()

	defaultSnap.withTesting(t).matchStandaloneSnapshot(value)
}

// Skip Wrapper of testing.Skip
//
// Keeps track which snapshots are getting skipped and not marked as obsolete.
func Skip(t TestingT, args ...any) {
	t.Helper()

	defaultSnap.withTesting(t).trackSkip()
	t.Skip(args...)
}

// Skipf Wrapper of testing.Skipf
//
// Keeps track which snapshots are getting skipped and not marked as obsolete.
func Skipf(t TestingT, format string, args ...any) {
	t.Helper()

	defaultSnap.withTesting(t).trackSkip()
	t.Skipf(format, args...)
}

// SkipNow Wrapper of testing.SkipNow
//
// Keeps track which snapshots are getting skipped and not marked as obsolete.
func SkipNow(t TestingT) {
	t.Helper()

	defaultSnap.withTesting(t).trackSkip()
	t.SkipNow()
}

// MatchJSON verifies the input matches the most recent snap file.
// Input can be a valid json string or []byte or whatever value can be passed
// successfully on `json.Marshal`.
//
//	MatchJSON(t, `{"user":"mock-user","age":10,"email":"mock@email.com"}`)
//	MatchJSON(t, []byte(`{"user":"mock-user","age":10,"email":"mock@email.com"}`))
//	MatchJSON(t, User{10, "mock-email"})
//
// MatchJSON also supports passing matchers as a third argument. Those matchers can act either as
// validators or placeholders for data that might change on each invocation e.g. dates.
//
//	MatchJSON(t, User{created: time.Now(), email: "mock-email"}, match.Any("created"))
func (c *Config) MatchJSON(t TestingT, input any, matchers ...matchers.JsonMatcher) {
	t.Helper()

	newSnap(c, t).matchJson(input, matchers...)
}

// MatchSnapshot verifies the values match the most recent snap file
// You can pass multiple values
//
//	MatchSnapshot(t, 10, "hello world")
//
// or call MatchSnapshot multiples times inside a test
//
//	MatchSnapshot(t, 10)
//	MatchSnapshot(t, "hello world")
//
// The difference is the latter will create multiple entries.
func (c *Config) MatchSnapshot(t TestingT, values ...any) {
	t.Helper()

	newSnap(c, t).matchSnapshot(values...)
}

// MatchStandaloneSnapshot verifies the value matches the most recent snap file
//
//	MatchStandaloneSnapshot(t, "Hello World")
//
// MatchStandaloneSnapshot creates one snapshot file per call.
//
// You can call MatchStandaloneSnapshot multiple times inside a test.
// It will create multiple snapshot files at `__snapshots__` folder by default.
func (c *Config) MatchStandaloneSnapshot(t TestingT, value any) {
	t.Helper()

	newSnap(c, t).matchStandaloneSnapshot(value)
}
