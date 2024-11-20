package _

import (
	"github.com/gkampitakis/go-snaps/snaps"
	"testing"
)

func TestBaseCallerNested(t *testing.T) {
	file := snaps.BaseCaller(0)

	snaps.Contains(t, file, "/snaps/contracts_test.go")
}

func testBaseCallerNested(t *testing.T) {
	file := snaps.BaseCaller(0)

	snaps.Contains(t, file, "/snaps/contracts_test.go")
}

func TestBaseCallerHelper(t *testing.T) {
	t.Helper()
	file := snaps.BaseCaller(0)

	snaps.Contains(t, file, "/snaps/contracts_test.go")
}

func TestBaseCaller(t *testing.T) {
	t.Run("should return correct baseCaller", func(t *testing.T) {
		var file string

		func() {
			file = snaps.BaseCaller(1)
		}()

		snaps.Contains(t, file, "/snaps/contracts_test.go")
	})

	t.Run("should return parent function", func(t *testing.T) {
		testBaseCallerNested(t)
	})

	t.Run("should return function's name", func(t *testing.T) {
		TestBaseCallerNested(t)
	})
}
