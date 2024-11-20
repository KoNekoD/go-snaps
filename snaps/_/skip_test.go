package _

import (
	"github.com/gkampitakis/go-snaps/snaps"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSkip(t *testing.T) {
	t.Run("should call Skip", func(t *testing.T) {
		t.Cleanup(func() {
			snaps.skippedTests = snaps.NewSyncSlice()
		})
		skipArgs := []any{1, 2, 3, 4, 5}

		mockT := snaps.NewMockTestingT(t)
		mockT.MockSkip = func(args ...any) {
			snaps.Equal(t, skipArgs, args)
		}
		mockT.MockLog = func(args ...any) {
			snaps.Equal(t, snaps.skippedMsg, args[0].(string))
		}
		mockT.MockSkip = func(...any) {}

		snaps.Skip(mockT, 1, 2, 3, 4, 5)

		snaps.Equal(t, []string{"mock-name"}, snaps.skippedTests.Values())
	})

	t.Run("should call Skipf", func(t *testing.T) {
		t.Cleanup(func() {
			snaps.skippedTests = snaps.NewSyncSlice()
		})

		mockT := snaps.NewMockTestingT(t)
		mockT.MockSkipf = func(format string, args ...any) {
			snaps.Equal(t, "mock", format)
			snaps.Equal(t, []any{1, 2, 3, 4, 5}, args)
		}
		mockT.MockLog = func(args ...any) {
			snaps.Equal(t, snaps.skippedMsg, args[0].(string))
		}
		mockT.MockSkipf = func(string, ...any) {}

		snaps.Skipf(mockT, "mock", 1, 2, 3, 4, 5)

		snaps.Equal(t, []string{"mock-name"}, snaps.skippedTests.Values())
	})

	t.Run("should call SkipNow", func(t *testing.T) {
		t.Cleanup(func() {
			snaps.skippedTests = snaps.NewSyncSlice()
		})

		mockT := snaps.NewMockTestingT(t)
		mockT.MockLog = func(args ...any) {
			snaps.Equal(t, snaps.skippedMsg, args[0].(string))
		}
		mockT.MockSkipNow = func() {}

		snaps.SkipNow(mockT)

		snaps.Equal(t, []string{"mock-name"}, snaps.skippedTests.Values())
	})

	t.Run("should be concurrent safe", func(t *testing.T) {
		t.Cleanup(func() {
			snaps.skippedTests = snaps.NewSyncSlice()
		})
		calledCount := atomic.Int64{}

		mockT := snaps.NewMockTestingT(t)
		mockT.MockLog = func(args ...any) {
			snaps.Equal(t, snaps.skippedMsg, args[0].(string))
		}
		mockT.MockSkipNow = func() {
			calledCount.Add(1)
		}
		wg := sync.WaitGroup{}

		for i := 0; i < 1000; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()
				snaps.SkipNow(mockT)
			}()
		}

		wg.Wait()

		snaps.Equal(t, 1000, len(snaps.skippedTests.Values()))
		snaps.Equal(t, 1000, calledCount.Load())
	})

	t.Run("testSkipped", func(t *testing.T) {
		t.Run("should return true if testID is not part of the 'runOnly'", func(t *testing.T) {
			runOnly := "TestMock"
			testID := "TestSkip/should_call_Skip - 1"

			received := TestSkipped(testID, runOnly)
			snaps.True(t, received)
		})

		t.Run("should return false if testID is part of 'runOnly'", func(t *testing.T) {
			runOnly := "TestMock"
			testID := "TestMock/Test/should_be_not_skipped - 2"

			received := TestSkipped(testID, runOnly)
			snaps.False(t, received)
		})

		t.Run(
			"should check if the parent is skipped and mark child tests as skipped",
			func(t *testing.T) {
				t.Cleanup(func() {
					snaps.skippedTests = snaps.NewSyncSlice()
				})

				runOnly := ""
				mockT := snaps.NewMockTestingT(t)
				mockT.MockName = func() string {
					return "TestMock/Skip"
				}
				mockT.MockLog = func(args ...any) {
					snaps.Equal(t, snaps.skippedMsg, args[0].(string))
				}
				mockT.MockSkipNow = func() {}

				// This is for populating skippedTests.values and following the normal flow
				snaps.SkipNow(mockT)

				snaps.True(t, TestSkipped("TestMock/Skip - 1000", runOnly))
				snaps.True(
					t,
					TestSkipped("TestMock/Skip/child_should_also_be_skipped", runOnly),
				)
				snaps.False(t, TestSkipped("TestAnotherTest", runOnly))
			},
		)

		t.Run("should not mark tests skipped if not not a child", func(t *testing.T) {
			t.Cleanup(func() {
				snaps.skippedTests = snaps.NewSyncSlice()
			})

			runOnly := ""
			mockT := snaps.NewMockTestingT(t)
			mockT.MockName = func() string {
				return "Test"
			}
			mockT.MockLog = func(args ...any) {
				snaps.Equal(t, snaps.skippedMsg, args[0].(string))
			}
			mockT.MockSkipNow = func() {}

			// This is for populating skippedTests.values and following the normal flow
			snaps.SkipNow(mockT)

			snaps.True(t, TestSkipped("Test - 1", runOnly))
			snaps.True(t, TestSkipped("Test/child - 1", runOnly))
			snaps.False(t, TestSkipped("TestMock - 1", runOnly))
			snaps.False(t, TestSkipped("TestMock/child - 1", runOnly))
		})

		t.Run("should use regex match for runOnly", func(t *testing.T) {
			snaps.False(t, TestSkipped("MyTest - 1", "Test"))
			snaps.True(t, TestSkipped("MyTest - 1", "^Test"))
		})
	})

	t.Run("isFileSkipped", func(t *testing.T) {
		t.Run("should return 'false'", func(t *testing.T) {
			snaps.False(t, IsFileSkipped("", "", ""))
		})

		t.Run("should return 'true' if test is not included in the test file", func(t *testing.T) {
			dir, _ := os.Getwd()

			snaps.Equal(
				t,
				true,
				IsFileSkipped(dir+"/__snapshots__", "skip_test.snap", "TestNonExistent"),
			)
		})

		t.Run("should return 'false' if test is included in the test file", func(t *testing.T) {
			dir, _ := os.Getwd()

			snaps.False(t, IsFileSkipped(dir+"/__snapshots__", "skip_test.snap", "TestSkip"))
		})

		t.Run("should use regex match for runOnly", func(t *testing.T) {
			dir, _ := os.Getwd()

			snaps.Equal(
				t,
				false,
				IsFileSkipped(dir+"/__snapshots__", "skip_test.snap", "TestSkip.*"),
			)
		})
	})
}
