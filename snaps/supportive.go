package snaps

import (
	"flag"
	"fmt"
	"github.com/gkampitakis/go-snaps/snaps/matchers"
	"strconv"
	"testing"
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

	defaultSnap.WithTesting(t).matchJson(input, matchers...)
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

	defaultSnap.WithTesting(t).matchSnapshot(values...)
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

	defaultSnap.WithTesting(t).matchStandaloneSnapshot(value)
}

// Clean runs checks for identifying obsolete snapshots and prints a Test Summary.
//
// Must be called in a TestMain
//
//	func TestMain(m *testing.M) {
//	 v := m.Run()
//
//	 // After all tests have run `go-snaps` can check for unused snapshots
//	 snaps.Clean(m)
//
//	 os.Exit(v)
//	}
//
// Clean also supports options for sorting the snapshots
//
//	func TestMain(m *testing.M) {
//	 v := m.Run()
//
//	 // After all tests have run `go-snaps` will sort snapshots
//	 snaps.Clean(m, snaps.CleanOpts{Sort: true})
//
//	 os.Exit(v)
//	}

type CleanOpts struct {
	// If set to true, `go-snaps` will sort the snapshots
	Sort bool
}

func Clean(m *testing.M, opts ...CleanOpts) {
	s := defaultSnap

	var opt CleanOpts
	if len(opts) != 0 {
		opt = opts[0]
	}

	// This is just for making sure Clean is called from TestMain
	_ = m
	runOnly := flag.Lookup("test.run").Value.String()
	count, _ := strconv.Atoi(flag.Lookup("test.count").Value.String())
	registeredStandaloneTests := s.occurrences(s.registry.registryCleanup, count, s.standaloneOccurrenceFMT)

	obsoleteFiles, usedFiles := s.examineFiles(s.registry.registryCleanup, registeredStandaloneTests, runOnly, shouldClean && !isCI)
	obsoleteTests, err := s.examineSnaps(s.registry.registryCleanup, usedFiles, runOnly, count, shouldClean && !isCI, opt.Sort && !isCI)
	if err != nil {
		fmt.Println(err)
		return
	}

	summary := s.summary(obsoleteFiles, obsoleteTests, len(s.getSkippedTests()), s.registry.testEvents, shouldClean && !isCI)
	if summary != "" {
		fmt.Println(s)
	}
}

// Skip Wrapper of testing.Skip
//
// Keeps track which snapshots are getting skipped and not marked as obsolete.
func Skip(t TestingT, args ...any) {
	t.Helper()

	defaultSnap.WithTesting(t).trackSkip()
	t.Skip(args...)
}

// Skipf Wrapper of testing.Skipf
//
// Keeps track which snapshots are getting skipped and not marked as obsolete.
func Skipf(t TestingT, format string, args ...any) {
	t.Helper()

	defaultSnap.WithTesting(t).trackSkip()
	t.Skipf(format, args...)
}

// SkipNow Wrapper of testing.SkipNow
//
// Keeps track which snapshots are getting skipped and not marked as obsolete.
func SkipNow(t TestingT) {
	t.Helper()

	defaultSnap.WithTesting(t).trackSkip()
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
