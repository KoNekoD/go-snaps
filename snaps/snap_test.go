package snaps

import (
	"errors"
	"github.com/gkampitakis/ciinfo"
	"github.com/gkampitakis/go-snaps/internal/test"
	"github.com/gkampitakis/go-snaps/snaps/colors"
	"github.com/gkampitakis/go-snaps/snaps/matchers"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestBaseCallerNested(t *testing.T) {
	file := defaultSnap.baseCaller(0)

	test.Contains(t, file, "/snaps/snap_test.go")
}

func testBaseCallerNested(t *testing.T) {
	file := defaultSnap.baseCaller(0)

	test.Contains(t, file, "/snaps/snap_test.go")
}

func TestBaseCallerHelper(t *testing.T) {
	t.Helper()
	file := defaultSnap.baseCaller(0)

	test.Contains(t, file, "/snaps/snap_test.go")
}

func TestBaseCaller(t *testing.T) {
	t.Run("should return correct baseCaller", func(t *testing.T) {
		var file string

		func() {
			file = defaultSnap.baseCaller(1)
		}()

		test.Contains(t, file, "/snaps/snap_test.go")
	})

	t.Run("should return parent function", func(t *testing.T) {
		testBaseCallerNested(t)
	})

	t.Run("should return function's name", func(t *testing.T) {
		TestBaseCallerNested(t)
	})
}

const jsonFilename = "matchJSON_test.snap"

func TestMatchJSON(t *testing.T) {
	t.Run("should validate json", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			input any
			err   string
		}{
			{
				name:  "string",
				input: "",
				err:   "invalid json",
			},
			{
				name:  "byte",
				input: []byte(`{"user"`),
				err:   "invalid json",
			},
			{
				name:  "struct",
				input: make(chan struct{}),
				err:   "json: unsupported type: chan struct {}",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				setupSnapshot(t, jsonFilename, false)

				mockT := test.NewMockTestingT(t)
				mockT.MockError = func(args ...any) {
					test.Equal(t, tc.err, (args[0].(error)).Error())
				}

				defaultSnap.WithTesting(mockT).matchJson(tc.input)
			})
		}
	})

	t.Run("matchers", func(t *testing.T) {
		t.Run("should apply matchers in order", func(t *testing.T) {
			setupSnapshot(t, jsonFilename, false)

			mockT := test.NewMockTestingT(t)
			mockT.MockLog = func(args ...any) { test.Equal(t, addedMsg, args[0].(string)) }

			c1 := func(val any) (any, error) {
				return map[string]any{"key2": nil}, nil
			}
			c2 := func(val any) (any, error) {
				return map[string]any{"key3": nil}, nil
			}
			c3 := func(val any) (any, error) {
				return map[string]any{"key4": nil}, nil
			}

			defaultSnap.WithTesting(mockT).matchJson(
				`{"key1":""}`,
				matchers.Custom("key1", c1),
				matchers.Custom("key1.key2", c2),
				matchers.Custom("key1.key2.key3", c3),
			)
		})

		t.Run("should aggregate errors from matchers", func(t *testing.T) {
			setupSnapshot(t, jsonFilename, false)

			mockT := test.NewMockTestingT(t)
			mockT.MockError = func(args ...any) {
				test.Equal(t,
					"\x1b[31;1m\n✕ match.Custom(\"age\") - mock error"+
						"\x1b[0m\x1b[31;1m\n✕ match.Any(\"missing.key.1\") - path does not exist"+
						"\x1b[0m\x1b[31;1m\n✕ match.Any(\"missing.key.2\") - path does not exist\x1b[0m",
					args[0],
				)
			}

			c := func(val any) (any, error) {
				return nil, errors.New("mock error")
			}
			defaultSnap.WithTesting(mockT).matchJson(
				`{"age":10}`,
				matchers.Custom("age", c),
				matchers.Any("missing.key.1", "missing.key.2"),
			)
		})
	})

	t.Run("if it's running on ci should skip creating snapshot", func(t *testing.T) {
		setupSnapshot(t, jsonFilename, true)

		mockT := test.NewMockTestingT(t)
		mockT.MockName = func() string {
			return "mock-name-check-file-not-found"
		}
		mockT.MockError = func(args ...any) {
			test.Equal(t, errSnapNotFound, args[0].(error))
		}

		defaultSnap.WithTesting(mockT).matchJson("{}")

		test.Equal(t, 1, defaultSnap.registry.testEvents[erred])
	})

	t.Run("should update snapshot when 'shouldUpdate'", func(t *testing.T) {
		snapPath := setupSnapshot(t, jsonFilename, false, true)

		printerExpectedCalls := []func(received any){
			func(received any) { test.Equal(t, addedMsg, received.(string)) },
			func(received any) { test.Equal(t, updatedMsg, received.(string)) },
		}
		mockT := test.NewMockTestingT(t)
		mockT.MockName = func() string {
			return "mock-name-should-update-when-should-update"
		}
		mockT.MockLog = func(args ...any) {
			printerExpectedCalls[0](args[0])

			// shift
			printerExpectedCalls = printerExpectedCalls[1:]
		}

		// First call for creating the snapshot
		defaultSnap.WithTesting(mockT).matchJson("{\"value\":\"hello world\"}")
		test.Equal(t, 1, defaultSnap.registry.testEvents[added])

		// Resetting registry to emulate the same MatchSnapshot call
		defaultSnap.registry = newSnapRegistry()

		// Second call with different params
		defaultSnap.WithTesting(mockT).matchJson("{\"value\":\"bye world\"}")

		test.Equal(
			t,
			"\n[mock-name - 1]\n{\n \"value\": \"bye world\"\n}\n---\n",
			test.GetFileContent(t, snapPath),
		)
		test.Equal(t, 1, defaultSnap.registry.testEvents[updated])
	})
}

const standaloneFilename = "mock-name_1.snap"

func TestMatchStandaloneSnapshot(t *testing.T) {
	t.Run("should create snapshot", func(t *testing.T) {
		snapPath := setupSnapshot(t, standaloneFilename, false)
		mockT := test.NewMockTestingT(t)
		mockT.MockLog = func(args ...any) { test.Equal(t, addedMsg, args[0].(string)) }

		MatchStandaloneSnapshot(mockT, "hello world")

		test.Equal(t, "hello world", test.GetFileContent(t, snapPath))
		test.Equal(t, 1, defaultSnap.registry.testEvents[added])
		// clean up function called

		registryKey := filepath.Join(
			filepath.Dir(snapPath),
			"mock-name_%d.snap",
		)
		test.Equal(t, 0, defaultSnap.registry.registryRunning[registryKey])
		test.Equal(t, 1, defaultSnap.registry.registryCleanup[registryKey])
	})

	t.Run("should pass tests with no diff", func(t *testing.T) {
		snapPath := setupSnapshot(t, standaloneFilename, false, false)

		printerExpectedCalls := []func(received any){
			func(received any) { test.Equal(t, addedMsg, received.(string)) },
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
		s.MatchStandaloneSnapshot(mockT, "hello world")
		test.Equal(t, 1, defaultSnap.registry.testEvents[added])

		// Resetting registry to emulate the same MatchStandaloneSnapshot call
		defaultSnap.registry = newSnapRegistry()

		// Second call with same params
		s.MatchStandaloneSnapshot(mockT, "hello world")

		test.Equal(t, "hello world", test.GetFileContent(t, snapPath))
		test.Equal(t, 1, defaultSnap.registry.testEvents[passed])
	})

	t.Run("if it's running on ci should skip creating snapshot", func(t *testing.T) {
		setupSnapshot(t, standaloneFilename, true)

		mockT := test.NewMockTestingT(t)
		mockT.MockError = func(args ...any) {
			test.Equal(t, errSnapNotFound, args[0].(error))
		}

		MatchStandaloneSnapshot(mockT, "hello world")

		test.Equal(t, 1, defaultSnap.registry.testEvents[erred])
	})

	t.Run("should return error when diff is found", func(t *testing.T) {
		setupSnapshot(t, standaloneFilename, false)

		printerExpectedCalls := []func(received any){
			func(received any) { test.Equal(t, addedMsg, received.(string)) },
			func(received any) { t.Error("should not be called 2nd time") },
		}
		mockT := test.NewMockTestingT(t)
		mockT.MockError = func(args ...any) {
			expected := "\n\x1b[38;5;52m\x1b[48;5;225m- Snapshot - 1\x1b[0m\n\x1b[38;5;22m\x1b[48;5;159m" +
				"+ Received + 1\x1b[0m\n\n\x1b[48;5;225m\x1b[38;5;52m- \x1b[0m\x1b[48;5;127m\x1b[38;5;255m" +
				"hello\x1b[0m\x1b[48;5;225m\x1b[38;5;52m world\x1b[0m\n\x1b[48;5;159m\x1b[38;5;22m" +
				"+ \x1b[0m\x1b[48;5;23m\x1b[38;5;255mbye\x1b[0m\x1b[48;5;159m\x1b[38;5;22m world\x1b[0m\n\n\x1b[2m" +
				"at " + filepath.FromSlash(
				"__snapshots__/mock-name_1.snap:1",
			) + "\n\x1b[0m"

			test.Equal(t, expected, args[0].(string))
		}
		mockT.MockLog = func(args ...any) {
			printerExpectedCalls[0](args[0])

			// shift
			printerExpectedCalls = printerExpectedCalls[1:]
		}

		// First call for creating the snapshot
		MatchStandaloneSnapshot(mockT, "hello world")
		test.Equal(t, 1, defaultSnap.registry.testEvents[added])

		// Resetting registry to emulate the same MatchStandaloneSnapshot call
		defaultSnap.registry = newSnapRegistry()

		// Second call with different data
		MatchStandaloneSnapshot(mockT, "bye world")
		test.Equal(t, 1, defaultSnap.registry.testEvents[erred])
	})

	t.Run("should update snapshot", func(t *testing.T) {
		t.Run("when 'updateVAR==true'", func(t *testing.T) {
			snapPath := setupSnapshot(t, standaloneFilename, false, true)

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
			MatchStandaloneSnapshot(mockT, "hello world")
			test.Equal(t, 1, defaultSnap.registry.testEvents[added])

			// Resetting registry to emulate the same MatchStandaloneSnapshot call
			defaultSnap.registry = newSnapRegistry()

			// Second call with different params
			MatchStandaloneSnapshot(mockT, "bye world")

			test.Equal(t, "bye world", test.GetFileContent(t, snapPath))
			test.Equal(t, 1, defaultSnap.registry.testEvents[updated])
		})

		t.Run("when config update", func(t *testing.T) {
			snapPath := setupSnapshot(t, standaloneFilename, false, false)

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
			s.MatchStandaloneSnapshot(mockT, "hello world")
			test.Equal(t, 1, defaultSnap.registry.testEvents[added])

			// Resetting registry to emulate the same MatchStandaloneSnapshot call
			defaultSnap.registry = newSnapRegistry()

			// Second call with different params
			s.MatchStandaloneSnapshot(mockT, "bye world")

			test.Equal(t, "bye world", test.GetFileContent(t, snapPath))
			test.Equal(t, 1, defaultSnap.registry.testEvents[updated])
		})
	})
}

func TestSkip(t *testing.T) {
	t.Run("should call Skip", func(t *testing.T) {
		t.Cleanup(func() {
			defaultSnap.skippedTests = make([]string, 0)
		})
		skipArgs := []any{1, 2, 3, 4, 5}

		mockT := test.NewMockTestingT(t)
		mockT.MockSkip = func(args ...any) {
			test.Equal(t, skipArgs, args)
		}
		mockT.MockLog = func(args ...any) {
			test.Equal(t, skippedMsg, args[0].(string))
		}
		mockT.MockSkip = func(...any) {}

		Skip(mockT, 1, 2, 3, 4, 5)

		test.Equal(t, []string{"mock-name"}, defaultSnap.skippedTests)
	})

	t.Run("should call Skipf", func(t *testing.T) {
		t.Cleanup(func() {
			defaultSnap.skippedTests = make([]string, 0)
		})

		mockT := test.NewMockTestingT(t)
		mockT.MockSkipf = func(format string, args ...any) {
			test.Equal(t, "mock", format)
			test.Equal(t, []any{1, 2, 3, 4, 5}, args)
		}
		mockT.MockLog = func(args ...any) {
			test.Equal(t, skippedMsg, args[0].(string))
		}
		mockT.MockSkipf = func(string, ...any) {}

		Skipf(mockT, "mock", 1, 2, 3, 4, 5)

		test.Equal(t, []string{"mock-name"}, defaultSnap.skippedTests)
	})

	t.Run("should call SkipNow", func(t *testing.T) {
		t.Cleanup(func() {
			defaultSnap.skippedTests = make([]string, 0)
		})

		mockT := test.NewMockTestingT(t)
		mockT.MockLog = func(args ...any) {
			test.Equal(t, skippedMsg, args[0].(string))
		}
		mockT.MockSkipNow = func() {}

		SkipNow(mockT)

		test.Equal(t, []string{"mock-name"}, defaultSnap.skippedTests)
	})

	t.Run("should be concurrent safe", func(t *testing.T) {
		t.Cleanup(func() {
			defaultSnap.skippedTests = make([]string, 0)
		})
		calledCount := atomic.Int64{}

		mockT := test.NewMockTestingT(t)
		mockT.MockLog = func(args ...any) {
			test.Equal(t, skippedMsg, args[0].(string))
		}
		mockT.MockSkipNow = func() {
			calledCount.Add(1)
		}
		wg := sync.WaitGroup{}

		for i := 0; i < 1000; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()
				SkipNow(mockT)
			}()
		}

		wg.Wait()

		test.Equal(t, 1000, len(defaultSnap.skippedTests))
		test.Equal(t, 1000, calledCount.Load())
	})
}

func TestSyncStandaloneRegistry(t *testing.T) {
	t.Run("should increment id on each call [concurrent safe]", func(t *testing.T) {
		wg := sync.WaitGroup{}

		for i := 0; i < 5; i++ {
			wg.Add(1)

			go func() {
				defaultSnap.getTestIdFromRegistry("/file/my_file_%d.snap", "./__snapshots__/my_file_%d.snap")
				wg.Done()
			}()
		}

		wg.Wait()

		snapPath, snapPathRel := defaultSnap.getTestIdFromRegistry(
			"/file/my_file_%d.snap",
			"./__snapshots__/my_file_%d.snap",
		)

		test.Equal(t, "/file/my_file_6.snap", snapPath)
		test.Equal(t, "./__snapshots__/my_file_6.snap", snapPathRel)

		snapPath, snapPathRel = defaultSnap.getTestIdFromRegistry(
			"/file/my_other_file_%d.snap",
			"./__snapshots__/my_other_file_%d.snap",
		)

		test.Equal(t, "/file/my_other_file_1.snap", snapPath)
		test.Equal(t, "./__snapshots__/my_other_file_1.snap", snapPathRel)
		test.Equal(t, defaultSnap.registry.registryCleanup, defaultSnap.registry.registryRunning)
	})

	t.Run("should reset running registry", func(t *testing.T) {
		wg := sync.WaitGroup{}

		for i := 0; i < 100; i++ {
			wg.Add(1)

			go func() {
				defaultSnap.getTestIdFromRegistry("/file/my_file_%d.snap", "./__snapshots__/my_file_%d.snap")
				wg.Done()
			}()
		}

		wg.Wait()

		defaultSnap.resetSnapPathInRegistry("/file/my_file_%d.snap")

		snapPath, snapPathRel := defaultSnap.getTestIdFromRegistry(
			"/file/my_file_%d.snap",
			"./__snapshots__/my_file_%d.snap",
		)

		// running registry start from 0 again
		test.Equal(t, "/file/my_file_1.snap", snapPath)
		test.Equal(t, "./__snapshots__/my_file_1.snap", snapPathRel)
		// cleanup registry still has 101
		test.Equal(t, 101, defaultSnap.registry.registryCleanup["/file/my_file_%d.snap"])
	})
}

func TestSnapshotPath(t *testing.T) {
	snapshotPathWrapper := func(c *Config, tName string) (snapPath, snapPathRel string) {
		// This is for emulating being called from a func so we can find the correct file
		// of the caller
		func() {
			func() {
				snapPath, snapPathRel = defaultSnap.snapshotPath()
			}()
		}()

		return
	}

	t.Run("should return standalone snapPath", func(t *testing.T) {
		snapPath, snapPathRel := snapshotPathWrapper(defaultConfig(), "my_test")

		test.HasSuffix(
			t,
			snapPath,
			filepath.FromSlash("/snaps/__snapshots__/my_test_%d.snap"),
		)
		test.Equal(
			t,
			filepath.FromSlash("__snapshots__/my_test_%d.snap"),
			snapPathRel,
		)
	})

	t.Run("should return standalone snapPath without '/'", func(t *testing.T) {
		snapPath, snapPathRel := snapshotPathWrapper(defaultConfig(), "TestFunction/my_test")

		test.HasSuffix(
			t,
			snapPath,
			filepath.FromSlash("/snaps/__snapshots__/TestFunction_my_test_%d.snap"),
		)
		test.Equal(
			t,
			filepath.FromSlash("__snapshots__/TestFunction_my_test_%d.snap"),
			snapPathRel,
		)
	})

	t.Run("should return standalone snapPath with overridden filename", func(t *testing.T) {
		snapPath, snapPathRel := snapshotPathWrapper(WithConfig(Filename("my_file"), Dir("my_snapshot_dir")), "my_test")

		test.HasSuffix(t, snapPath, filepath.FromSlash("/snaps/my_snapshot_dir/my_file_%d.snap"))
		test.Equal(t, filepath.FromSlash("my_snapshot_dir/my_file_%d.snap"), snapPathRel)
	})

	t.Run(
		"should return standalone snapPath with overridden filename and extension",
		func(t *testing.T) {
			snapPath, snapPathRel := snapshotPathWrapper(WithConfig(Filename("my_file"), Dir("my_snapshot_dir"), Ext(".txt")), "my_test")

			test.HasSuffix(t, snapPath, filepath.FromSlash("/snaps/my_snapshot_dir/my_file_%d.snap.txt"))
			test.Equal(t, filepath.FromSlash("my_snapshot_dir/my_file_%d.snap.txt"), snapPathRel)
		},
	)
}

const (
	fileName = "matchSnapshot_test.snap"
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
		_ = os.Remove(snapPath)
		defaultSnap.registry = newSnapRegistry()
		defaultSnap.registry.testEvents = make(map[uint8]int)
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
		test.Equal(t, 1, defaultSnap.registry.testEvents[added])
		// clean up function called
		test.Equal(t, 0, defaultSnap.registry.registryRunning["mock-name"])
		test.Equal(t, 1, defaultSnap.registry.registryCleanup["mock-name"])
	})

	t.Run("if it's running on ci should skip creating snapshot", func(t *testing.T) {
		setupSnapshot(t, fileName, true)

		mockT := test.NewMockTestingT(t)
		mockT.MockError = func(args ...any) {
			test.Equal(t, errSnapNotFound, args[0].(error))
		}

		MatchSnapshot(mockT, 10, "hello world")

		test.Equal(t, 1, defaultSnap.registry.testEvents[erred])
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
				"__snapshots__/matchSnapshot_test.snap:2",
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
		test.Equal(t, 1, defaultSnap.registry.testEvents[added])

		// Resetting registry to emulate the same MatchSnapshot call
		defaultSnap.registry = newSnapRegistry()

		// Second call with different params
		MatchSnapshot(mockT, 100, "bye world")
		test.Equal(t, 1, defaultSnap.registry.testEvents[erred])
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
			test.Equal(t, 1, defaultSnap.registry.testEvents[added])

			// Resetting registry to emulate the same MatchSnapshot call
			defaultSnap.registry = newSnapRegistry()

			// Second call with different params
			MatchSnapshot(mockT, 100, "bye world")

			test.Equal(
				t,
				"\n[mock-name - 1]\nint(100)\nbye world\n---\n",
				test.GetFileContent(t, snapPath),
			)
			test.Equal(t, 1, defaultSnap.registry.testEvents[updated])
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
			test.Equal(t, 1, defaultSnap.registry.testEvents[added])

			// Resetting registry to emulate the same MatchSnapshot call
			defaultSnap.registry = newSnapRegistry()

			// Second call with different params
			s.MatchSnapshot(mockT, 100, "bye world")

			test.Equal(
				t,
				"\n[mock-name - 1]\nint(100)\nbye world\n---\n",
				test.GetFileContent(t, snapPath),
			)
			test.Equal(t, 1, defaultSnap.registry.testEvents[updated])
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
				"__snapshots__/matchSnapshot_test.snap:2",
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
		defaultSnap.registry = newSnapRegistry()

		// Second call with different params
		MatchSnapshot(mockT, 100, "bye world----", "--")
	})
}
