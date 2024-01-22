package snaps

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gkampitakis/ciinfo"
	"github.com/gkampitakis/go-snaps/internal/colors"
	"github.com/gkampitakis/go-snaps/internal/test"
)

const (
	fileName = "matchSnapshot_test.snap"
	mockSnap = `

[Test_1/TestSimple - 1]
int(1)
string hello world 1 1 1
---

[Test_3/TestSimple - 1]
int(100)
string hello world 1 3 1
---

[Test_3/TestSimple - 2]
int(1000)
string hello world 1 3 2
---

`
)

func setupSnapshot(t *testing.T, file string, ci bool, update ...bool) string {
	t.Helper()
	dir, _ := os.Getwd()
	snapPath := filepath.Join(dir, "__snapshots__", file)
	isCI = ci
	updateVARPrev := updateVAR
	updateVAR = ""
	if len(update) > 0 && update[0] {
		updateVAR = "true"
	}

	t.Cleanup(func() {
		os.Remove(snapPath)
		testsRegistry = newRegistry()
		testEvents = newTestEvents()
		isCI = ciinfo.IsCI
		updateVAR = updateVARPrev
	})

	_, err := os.Stat(snapPath)
	// This is for checking we are starting with a clean state testing
	test.True(t, errors.Is(err, os.ErrNotExist))

	return snapPath
}

func TestMatchSnapshot(t *testing.T) {
	t.Run("should create snapshot", func(t *testing.T) {
		snapPath := setupSnapshot(t, fileName, false)
		mockT := test.NewMockTestingT(t)
		mockT.MockLog = func(args ...any) { test.Equal(t, addedMsg, args[0].(string)) }

		MatchSnapshot(mockT, 10, "hello world")

		test.Equal(
			t,
			"\n[mock-name - 1]\nint(10)\nhello world\n---\n",
			test.GetFileContent(t, snapPath),
		)
		test.Equal(t, 1, testEvents.items[added])
	})

	t.Run("if it's running on ci should skip", func(t *testing.T) {
		setupSnapshot(t, fileName, true)

		mockT := test.NewMockTestingT(t)
		mockT.MockError = func(args ...any) {
			test.Equal(t, errSnapNotFound, args[0].(error))
		}

		MatchSnapshot(mockT, 10, "hello world")

		test.Equal(t, 1, testEvents.items[erred])
	})

	t.Run("should return error when diff is found", func(t *testing.T) {
		setupSnapshot(t, fileName, false)

		printerExpectedCalls := []func(received any){
			func(received any) { test.Equal(t, addedMsg, received.(string)) },
			func(received any) { t.Error("should not be called 2nd time") },
		}
		mockT := test.NewMockTestingT(t)
		mockT.MockError = func(args ...any) {
			expected := "\n\x1b[38;5;52m\x1b[48;5;225m- Snapshot - 2\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m" +
				"+ Received + 2\x1b[0m\n\n\x1b[38;5;52m\x1b[48;5;225m- int(10)\x1b[0m\n\x1b[38;5;52m\x1b[48;5;225m" +
				"- hello world\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m+ int(100)\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m" +
				"+ bye world\x1b[0m\n\n\x1b[2mat " + filepath.FromSlash(
				"../__snapshots__/matchSnapshot_test.snap:2",
			) +
				"\n\x1b[0m"

			test.Equal(t, expected, args[0].(string))
		}
		mockT.MockLog = func(args ...any) {
			printerExpectedCalls[0](args[0])

			// shift
			printerExpectedCalls = printerExpectedCalls[1:]
		}

		// First call for creating the snapshot
		MatchSnapshot(mockT, 10, "hello world")
		test.Equal(t, 1, testEvents.items[added])

		// Resetting registry to emulate the same MatchSnapshot call
		testsRegistry = newRegistry()

		// Second call with different params
		MatchSnapshot(mockT, 100, "bye world")
		test.Equal(t, 1, testEvents.items[erred])
	})

	t.Run("should update snapshot", func(t *testing.T) {
		t.Run("when 'updateVAR==true'", func(t *testing.T) {
			snapPath := setupSnapshot(t, fileName, false, true)

			printerExpectedCalls := []func(received any){
				func(received any) { test.Equal(t, addedMsg, received.(string)) },
				func(received any) { test.Equal(t, updatedMsg, received.(string)) },
				func(received any) { t.Error("should not be called 3rd time") },
			}
			mockT := test.NewMockTestingT(t)
			mockT.MockLog = func(args ...any) {
				printerExpectedCalls[0](args[0])

				// shift
				printerExpectedCalls = printerExpectedCalls[1:]
			}

			// First call for creating the snapshot
			MatchSnapshot(mockT, 10, "hello world")
			test.Equal(t, 1, testEvents.items[added])

			// Resetting registry to emulate the same MatchSnapshot call
			testsRegistry = newRegistry()

			// Second call with different params
			MatchSnapshot(mockT, 100, "bye world")

			test.Equal(
				t,
				"\n[mock-name - 1]\nint(100)\nbye world\n---\n",
				test.GetFileContent(t, snapPath),
			)
			test.Equal(t, 1, testEvents.items[updated])
		})

		t.Run("when config update", func(t *testing.T) {
			snapPath := setupSnapshot(t, fileName, false, false)

			printerExpectedCalls := []func(received any){
				func(received any) { test.Equal(t, addedMsg, received.(string)) },
				func(received any) { test.Equal(t, updatedMsg, received.(string)) },
				func(received any) { t.Error("should not be called 3rd time") },
			}
			mockT := test.NewMockTestingT(t)
			mockT.MockLog = func(args ...any) {
				printerExpectedCalls[0](args[0])

				// shift
				printerExpectedCalls = printerExpectedCalls[1:]
			}

			s := WithConfig(Update(true))
			// First call for creating the snapshot
			s.MatchSnapshot(mockT, 10, "hello world")
			test.Equal(t, 1, testEvents.items[added])

			// Resetting registry to emulate the same MatchSnapshot call
			testsRegistry = newRegistry()

			// Second call with different params
			s.MatchSnapshot(mockT, 100, "bye world")

			test.Equal(
				t,
				"\n[mock-name - 1]\nint(100)\nbye world\n---\n",
				test.GetFileContent(t, snapPath),
			)
			test.Equal(t, 1, testEvents.items[updated])
		})
	})

	t.Run("should print warning if no params provided", func(t *testing.T) {
		mockT := test.NewMockTestingT(t)
		mockT.MockLog = func(args ...any) {
			test.Equal(
				t,
				colors.Sprint(colors.Yellow, "[warning] MatchSnapshot call without params\n"),
				args[0].(string),
			)
		}

		MatchSnapshot(mockT)
	})

	t.Run("diff should not print the escaped characters", func(t *testing.T) {
		setupSnapshot(t, fileName, false)

		printerExpectedCalls := []func(received any){
			func(received any) { test.Equal(t, addedMsg, received.(string)) },
			func(received any) { t.Error("should not be called 2nd time") },
		}
		mockT := test.NewMockTestingT(t)
		mockT.MockError = func(args ...any) {
			expected := "\n\x1b[38;5;52m\x1b[48;5;225m- Snapshot - 3\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m" +
				"+ Received + 3\x1b[0m\n\n\x1b[38;5;52m\x1b[48;5;225m- int(10)\x1b[0m\n\x1b[38;5;52m\x1b[48;5;225m" +
				"- hello world----\x1b[0m\n\x1b[38;5;52m\x1b[48;5;225m- ---\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m" +
				"+ int(100)\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m+ bye world----\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m" +
				"+ --\x1b[0m\n\n\x1b[2mat " + filepath.FromSlash(
				"../__snapshots__/matchSnapshot_test.snap:2",
			) +
				"\n\x1b[0m"

			test.Equal(t, expected, args[0].(string))
		}
		mockT.MockLog = func(args ...any) {
			printerExpectedCalls[0](args[0])

			// shift
			printerExpectedCalls = printerExpectedCalls[1:]
		}

		// First call for creating the snapshot ( adding ending chars inside the diff )
		MatchSnapshot(mockT, 10, "hello world----", endSequence)

		// Resetting registry to emulate the same MatchSnapshot call
		testsRegistry = newRegistry()

		// Second call with different params
		MatchSnapshot(mockT, 100, "bye world----", "--")
	})
}
