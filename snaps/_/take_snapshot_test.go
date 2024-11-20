package _

import (
	"errors"
	"github.com/gkampitakis/go-snaps/snaps"
	"os"
	"path/filepath"
	"testing"

	"github.com/gkampitakis/ciinfo"
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
	ci.IsCI = ci
	updateVARPrev := snaps.UpdateVAR
	snaps.UpdateVAR = ""
	if len(update) > 0 && update[0] {
		snaps.UpdateVAR = "true"
	}

	t.Cleanup(func() {
		os.Remove(snapPath)
		snaps.registry = snaps.newStandaloneRegistry()
		snaps.testEvents = snaps.newTestEvents()
		ci.IsCI = ciinfo.IsCI
		snaps.UpdateVAR = updateVARPrev
	})

	_, err := os.Stat(snapPath)
	// This is for checking we are starting with a clean state testing
	snaps.True(t, errors.Is(err, os.ErrNotExist))

	return snapPath
}

func TestMatchSnapshot(t *testing.T) {
	t.Run("should create snapshot", func(t *testing.T) {
		snapPath := setupSnapshot(t, fileName, false)
		mockT := snaps.NewMockTestingT(t)
		mockT.MockLog = func(args ...any) { snaps.Equal(t, snaps.addedMsg, args[0].(string)) }

		snaps.MatchSnapshot(mockT, 10, "hello world")

		snaps.Equal(
			t,
			"\n[mock-name - 1]\nint(10)\nhello world\n---\n",
			snaps.GetFileContent(t, snapPath),
		)
		snaps.Equal(t, 1, snaps.testEvents.items[snaps.added])
		// clean up function called
		snaps.Equal(t, 0, snaps.registry.running["mock-name"])
		snaps.Equal(t, 1, snaps.registry.cleanup["mock-name"])
	})

	t.Run("if it's running on ci should skip creating snapshot", func(t *testing.T) {
		setupSnapshot(t, fileName, true)

		mockT := snaps.NewMockTestingT(t)
		mockT.MockError = func(args ...any) {
			snaps.Equal(t, snaps.errSnapNotFound, args[0].(error))
		}

		snaps.MatchSnapshot(mockT, 10, "hello world")

		snaps.Equal(t, 1, snaps.testEvents.items[snaps.erred])
	})

	t.Run("should return error when diff is found", func(t *testing.T) {
		setupSnapshot(t, fileName, false)

		printerExpectedCalls := []func(received any){
			func(received any) { snaps.Equal(t, snaps.addedMsg, received.(string)) },
			func(received any) { t.Error("should not be called 2nd time") },
		}
		mockT := snaps.NewMockTestingT(t)
		mockT.MockError = func(args ...any) {
			expected := "\n\x1b[38;5;52m\x1b[48;5;225m- Snapshot - 2\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m" +
				"+ Received + 2\x1b[0m\n\n\x1b[38;5;52m\x1b[48;5;225m- int(10)\x1b[0m\n\x1b[38;5;52m\x1b[48;5;225m" +
				"- hello world\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m+ int(100)\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m" +
				"+ bye world\x1b[0m\n\n\x1b[2mat " + filepath.FromSlash(
				"__snapshots__/matchSnapshot_test.snap:2",
			) +
				"\n\x1b[0m"

			snaps.Equal(t, expected, args[0].(string))
		}
		mockT.MockLog = func(args ...any) {
			printerExpectedCalls[0](args[0])

			// shift
			printerExpectedCalls = printerExpectedCalls[1:]
		}

		// First call for creating the snapshot
		snaps.MatchSnapshot(mockT, 10, "hello world")
		snaps.Equal(t, 1, snaps.testEvents.items[snaps.added])

		// Resetting registry to emulate the same MatchSnapshot call
		snaps.registry = snaps.newStandaloneRegistry()

		// Second call with different params
		snaps.MatchSnapshot(mockT, 100, "bye world")
		snaps.Equal(t, 1, snaps.testEvents.items[snaps.erred])
	})

	t.Run("should update snapshot", func(t *testing.T) {
		t.Run("when 'updateVAR==true'", func(t *testing.T) {
			snapPath := setupSnapshot(t, fileName, false, true)

			printerExpectedCalls := []func(received any){
				func(received any) { snaps.Equal(t, snaps.addedMsg, received.(string)) },
				func(received any) { snaps.Equal(t, snaps.updatedMsg, received.(string)) },
				func(received any) { t.Error("should not be called 3rd time") },
			}
			mockT := snaps.NewMockTestingT(t)
			mockT.MockLog = func(args ...any) {
				printerExpectedCalls[0](args[0])

				// shift
				printerExpectedCalls = printerExpectedCalls[1:]
			}

			// First call for creating the snapshot
			snaps.MatchSnapshot(mockT, 10, "hello world")
			snaps.Equal(t, 1, snaps.testEvents.items[snaps.added])

			// Resetting registry to emulate the same MatchSnapshot call
			snaps.registry = snaps.newStandaloneRegistry()

			// Second call with different params
			snaps.MatchSnapshot(mockT, 100, "bye world")

			snaps.Equal(
				t,
				"\n[mock-name - 1]\nint(100)\nbye world\n---\n",
				snaps.GetFileContent(t, snapPath),
			)
			snaps.Equal(t, 1, snaps.testEvents.items[snaps.updated])
		})

		t.Run("when config update", func(t *testing.T) {
			snapPath := setupSnapshot(t, fileName, false, false)

			printerExpectedCalls := []func(received any){
				func(received any) { snaps.Equal(t, snaps.addedMsg, received.(string)) },
				func(received any) { snaps.Equal(t, snaps.updatedMsg, received.(string)) },
				func(received any) { t.Error("should not be called 3rd time") },
			}
			mockT := snaps.NewMockTestingT(t)
			mockT.MockLog = func(args ...any) {
				printerExpectedCalls[0](args[0])

				// shift
				printerExpectedCalls = printerExpectedCalls[1:]
			}

			s := snaps.WithConfig(snaps.Update(true))
			// First call for creating the snapshot
			s.MatchSnapshot(mockT, 10, "hello world")
			snaps.Equal(t, 1, snaps.testEvents.items[snaps.added])

			// Resetting registry to emulate the same MatchSnapshot call
			snaps.registry = snaps.newStandaloneRegistry()

			// Second call with different params
			s.MatchSnapshot(mockT, 100, "bye world")

			snaps.Equal(
				t,
				"\n[mock-name - 1]\nint(100)\nbye world\n---\n",
				snaps.GetFileContent(t, snapPath),
			)
			snaps.Equal(t, 1, snaps.testEvents.items[snaps.updated])
		})
	})

	t.Run("should print warning if no params provided", func(t *testing.T) {
		mockT := snaps.NewMockTestingT(t)
		mockT.MockLog = func(args ...any) {
			snaps.Equal(
				t,
				snaps.Sprint(snaps.Yellow, "[warning] MatchSnapshot call without params\n"),
				args[0].(string),
			)
		}

		snaps.MatchSnapshot(mockT)
	})

	t.Run("diff should not print the escaped characters", func(t *testing.T) {
		setupSnapshot(t, fileName, false)

		printerExpectedCalls := []func(received any){
			func(received any) { snaps.Equal(t, snaps.addedMsg, received.(string)) },
			func(received any) { t.Error("should not be called 2nd time") },
		}
		mockT := snaps.NewMockTestingT(t)
		mockT.MockError = func(args ...any) {
			expected := "\n\x1b[38;5;52m\x1b[48;5;225m- Snapshot - 3\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m" +
				"+ Received + 3\x1b[0m\n\n\x1b[38;5;52m\x1b[48;5;225m- int(10)\x1b[0m\n\x1b[38;5;52m\x1b[48;5;225m" +
				"- hello world----\x1b[0m\n\x1b[38;5;52m\x1b[48;5;225m- ---\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m" +
				"+ int(100)\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m+ bye world----\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m" +
				"+ --\x1b[0m\n\n\x1b[2mat " + filepath.FromSlash(
				"__snapshots__/matchSnapshot_test.snap:2",
			) +
				"\n\x1b[0m"

			snaps.Equal(t, expected, args[0].(string))
		}
		mockT.MockLog = func(args ...any) {
			printerExpectedCalls[0](args[0])

			// shift
			printerExpectedCalls = printerExpectedCalls[1:]
		}

		// First call for creating the snapshot ( adding ending chars inside the diff )
		snaps.MatchSnapshot(mockT, 10, "hello world----", snaps.endSequence)

		// Resetting registry to emulate the same MatchSnapshot call
		snaps.registry = snaps.newStandaloneRegistry()

		// Second call with different params
		snaps.MatchSnapshot(mockT, 100, "bye world----", "--")
	})
}
