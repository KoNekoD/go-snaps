package _

import (
	"github.com/gkampitakis/go-snaps/snaps"
	"path/filepath"
	"testing"
)

const standaloneFilename = "mock-name_1.snap"

func TestMatchStandaloneSnapshot(t *testing.T) {
	t.Run("should create snapshot", func(t *testing.T) {
		snapPath := snaps.setupSnapshot(t, standaloneFilename, false)
		mockT := snaps.NewMockTestingT(t)
		mockT.MockLog = func(args ...any) { snaps.Equal(t, snaps.addedMsg, args[0].(string)) }

		snaps.MatchStandaloneSnapshot(mockT, "hello world")

		snaps.Equal(t, "hello world", snaps.GetFileContent(t, snapPath))
		snaps.Equal(t, 1, snaps.testEvents.items[snaps.added])
		// clean up function called

		registryKey := filepath.Join(
			filepath.Dir(snapPath),
			"mock-name_%d.snap",
		)
		snaps.Equal(t, 0, snaps.registry.running[registryKey])
		snaps.Equal(t, 1, snaps.registry.cleanup[registryKey])
	})

	t.Run("should pass tests with no diff", func(t *testing.T) {
		snapPath := snaps.setupSnapshot(t, standaloneFilename, false, false)

		printerExpectedCalls := []func(received any){
			func(received any) { snaps.Equal(t, snaps.addedMsg, received.(string)) },
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
		s.MatchStandaloneSnapshot(mockT, "hello world")
		snaps.Equal(t, 1, snaps.testEvents.items[snaps.added])

		// Resetting registry to emulate the same MatchStandaloneSnapshot call
		snaps.registry = snaps.newStandaloneRegistry()

		// Second call with same params
		s.MatchStandaloneSnapshot(mockT, "hello world")

		snaps.Equal(t, "hello world", snaps.GetFileContent(t, snapPath))
		snaps.Equal(t, 1, snaps.testEvents.items[snaps.passed])
	})

	t.Run("if it's running on ci should skip creating snapshot", func(t *testing.T) {
		snaps.setupSnapshot(t, standaloneFilename, true)

		mockT := snaps.NewMockTestingT(t)
		mockT.MockError = func(args ...any) {
			snaps.Equal(t, snaps.errSnapNotFound, args[0].(error))
		}

		snaps.MatchStandaloneSnapshot(mockT, "hello world")

		snaps.Equal(t, 1, snaps.testEvents.items[snaps.erred])
	})

	t.Run("should return error when diff is found", func(t *testing.T) {
		snaps.setupSnapshot(t, standaloneFilename, false)

		printerExpectedCalls := []func(received any){
			func(received any) { snaps.Equal(t, snaps.addedMsg, received.(string)) },
			func(received any) { t.Error("should not be called 2nd time") },
		}
		mockT := snaps.NewMockTestingT(t)
		mockT.MockError = func(args ...any) {
			expected := "\n\x1b[38;5;52m\x1b[48;5;225m- Snapshot - 1\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m" +
				"+ Received + 1\x1b[0m\n\n\x1b[48;5;225m\x1b[38;5;52m- \x1b[0m\x1b[48;5;127m\x1b[38;5;255m" +
				"hello\x1b[0m\x1b[48;5;225m\x1b[38;5;52m world\x1b[0m\n\x1b[48;5;159m\x1b[38;5;22m" +
				"+ \x1b[0m\x1b[48;5;23m\x1b[38;5;255mbye\x1b[0m\x1b[48;5;159m\x1b[38;5;22m world\x1b[0m\n\n\x1b[2m" +
				"at " + filepath.FromSlash(
				"__snapshots__/mock-name_1.snap:1",
			) + "\n\x1b[0m"

			snaps.Equal(t, expected, args[0].(string))
		}
		mockT.MockLog = func(args ...any) {
			printerExpectedCalls[0](args[0])

			// shift
			printerExpectedCalls = printerExpectedCalls[1:]
		}

		// First call for creating the snapshot
		snaps.MatchStandaloneSnapshot(mockT, "hello world")
		snaps.Equal(t, 1, snaps.testEvents.items[snaps.added])

		// Resetting registry to emulate the same MatchStandaloneSnapshot call
		snaps.registry = snaps.newStandaloneRegistry()

		// Second call with different data
		snaps.MatchStandaloneSnapshot(mockT, "bye world")
		snaps.Equal(t, 1, snaps.testEvents.items[snaps.erred])
	})

	t.Run("should update snapshot", func(t *testing.T) {
		t.Run("when 'updateVAR==true'", func(t *testing.T) {
			snapPath := snaps.setupSnapshot(t, standaloneFilename, false, true)

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
			snaps.MatchStandaloneSnapshot(mockT, "hello world")
			snaps.Equal(t, 1, snaps.testEvents.items[snaps.added])

			// Resetting registry to emulate the same MatchStandaloneSnapshot call
			snaps.registry = snaps.newStandaloneRegistry()

			// Second call with different params
			snaps.MatchStandaloneSnapshot(mockT, "bye world")

			snaps.Equal(t, "bye world", snaps.GetFileContent(t, snapPath))
			snaps.Equal(t, 1, snaps.testEvents.items[snaps.updated])
		})

		t.Run("when config update", func(t *testing.T) {
			snapPath := snaps.setupSnapshot(t, standaloneFilename, false, false)

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
			s.MatchStandaloneSnapshot(mockT, "hello world")
			snaps.Equal(t, 1, snaps.testEvents.items[snaps.added])

			// Resetting registry to emulate the same MatchStandaloneSnapshot call
			snaps.registry = snaps.newStandaloneRegistry()

			// Second call with different params
			s.MatchStandaloneSnapshot(mockT, "bye world")

			snaps.Equal(t, "bye world", snaps.GetFileContent(t, snapPath))
			snaps.Equal(t, 1, snaps.testEvents.items[snaps.updated])
		})
	})
}
