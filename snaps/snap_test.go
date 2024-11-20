package snaps

import (
	"errors"
	"fmt"
	"github.com/gkampitakis/ciinfo"
	"github.com/gkampitakis/go-snaps/internal/test"
	"github.com/gkampitakis/go-snaps/snaps/colors"
	"github.com/gkampitakis/go-snaps/snaps/matchers"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
)

// loadMockSnap loads a mock snap from the testdata directory
func loadMockSnap(t *testing.T, name string) []byte {
	t.Helper()
	snap, err := os.ReadFile(fmt.Sprintf("testdata/%s", name))
	if err != nil {
		t.Fatal(err)
	}

	return snap
}

func setupTempExamineFiles(
	t *testing.T,
	mockSnap1, mockSnap2 []byte,
) (map[string]int, string, string) {
	t.Helper()
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	files := []struct {
		name string
		data []byte
	}{
		{name: filepath.FromSlash(dir1 + "/test1.snap"), data: mockSnap1},
		{name: filepath.FromSlash(dir2 + "/test2.snap"), data: mockSnap2},
		{name: filepath.FromSlash(dir1 + "/obsolete1.snap"), data: []byte{}},
		{name: filepath.FromSlash(dir2 + "/obsolete2.snap"), data: []byte{}},
		{name: filepath.FromSlash(dir2 + "/should_not_delete.txt"), data: []byte{}},
		{name: filepath.FromSlash(dir1 + "TestSomething_my_test_1.snap"), data: []byte{}},
		{name: filepath.FromSlash(dir1 + "TestSomething_my_test_2.snap"), data: []byte{}},
		{name: filepath.FromSlash(dir1 + "TestSomething_my_test_3.snap"), data: []byte{}},
		{name: filepath.FromSlash(dir2 + "TestAnotherThing_my_test_1.snap"), data: []byte{}},
		{name: filepath.FromSlash(dir2 + "TestAnotherThing_my_simple_test_1.snap"), data: []byte{}},
		{name: filepath.FromSlash(dir2 + "TestAnotherThing_my_simple_test_2.snap"), data: []byte{}},
	}

	for _, file := range files {
		err := os.WriteFile(file.name, file.data, os.ModePerm)
		if err != nil {
			t.Fatal(err)
		}
	}

	tests := map[string]int{
		files[0].name: 1,
		files[1].name: 1,
	}

	return tests, dir1, dir2
}

func TestExamineFiles(t *testing.T) {
	t.Run("should parse files", func(t *testing.T) {
		tests, dir1, dir2 := setupTempExamineFiles(
			t,
			loadMockSnap(t, "mock-snap-1"),
			loadMockSnap(t, "mock-snap-2"),
		)
		obsolete, used := defaultSnap.examineFiles(tests, map[string]struct{}{
			dir1 + "TestSomething_my_test_1.snap":           {},
			dir2 + "TestAnotherThing_my_simple_test_1.snap": {},
		}, "", false)

		obsoleteExpected := []string{
			filepath.FromSlash(dir1 + "/obsolete1.snap"),
			filepath.FromSlash(dir2 + "/obsolete2.snap"),
			filepath.FromSlash(dir1 + "TestSomething_my_test_2.snap"),
			filepath.FromSlash(dir1 + "TestSomething_my_test_3.snap"),
			filepath.FromSlash(dir2 + "TestAnotherThing_my_test_1.snap"),
			filepath.FromSlash(dir2 + "TestAnotherThing_my_simple_test_2.snap"),
		}
		usedExpected := []string{filepath.FromSlash(dir1 + "/test1.snap"), filepath.FromSlash(dir2 + "/test2.snap")}

		// Parse files uses maps so order of strings cannot be guaranteed
		sort.Strings(obsoleteExpected)
		sort.Strings(usedExpected)
		sort.Strings(obsolete)
		sort.Strings(used)

		test.Equal(t, obsoleteExpected, obsolete)
		test.Equal(t, usedExpected, used)
	})

	t.Run("should remove outdated files", func(t *testing.T) {
		tests, dir1, dir2 := setupTempExamineFiles(
			t,
			loadMockSnap(t, "mock-snap-1"),
			loadMockSnap(t, "mock-snap-2"),
		)
		defaultSnap.examineFiles(tests, map[string]struct{}{
			dir1 + "TestSomething_my_test_1.snap":           {},
			dir2 + "TestAnotherThing_my_simple_test_1.snap": {},
		}, "", true)

		for _, obsoleteFilename := range []string{
			dir1 + "obsolete1.snap",
			dir2 + "obsolete2.snap",
			dir1 + "TestSomething_my_test_2.snap",
			dir1 + "TestSomething_my_test_3.snap",
			dir2 + "TestAnotherThing_my_test_1.snap",
			dir2 + "TestAnotherThing_my_simple_test_2.snap",
		} {
			if _, err := os.Stat(filepath.FromSlash(obsoleteFilename)); !errors.Is(
				err,
				os.ErrNotExist,
			) {
				t.Errorf("obsolete file %s not removed", obsoleteFilename)
			}
		}
	})
}

func TestExamineSnaps(t *testing.T) {
	t.Run("should report no obsolete snapshots", func(t *testing.T) {
		tests, dir1, dir2 := setupTempExamineFiles(
			t,
			loadMockSnap(t, "mock-snap-1"),
			loadMockSnap(t, "mock-snap-2"),
		)
		used := []string{
			filepath.FromSlash(dir1 + "/test1.snap"),
			filepath.FromSlash(dir2 + "/test2.snap"),
		}

		obsolete, err := defaultSnap.examineSnaps(tests, used, "", 1, false, false)

		test.Equal(t, []string{}, obsolete)
		test.NoError(t, err)
	})

	t.Run("should report two obsolete snapshots and not change content", func(t *testing.T) {
		mockSnap1 := loadMockSnap(t, "mock-snap-1")
		mockSnap2 := loadMockSnap(t, "mock-snap-2")
		tests, dir1, dir2 := setupTempExamineFiles(t, mockSnap1, mockSnap2)
		used := []string{
			filepath.FromSlash(dir1 + "/test1.snap"),
			filepath.FromSlash(dir2 + "/test2.snap"),
		}

		// Reducing test occurrence to 1 meaning the second test was removed ( testid - 2 )
		tests["TestDir1_3__TestSimple"] = 1
		// Removing the test entirely
		delete(tests, "TestDir2_2/TestSimple")

		obsolete, err := defaultSnap.examineSnaps(tests, used, "", 1, false, false)
		content1 := test.GetFileContent(t, used[0])
		content2 := test.GetFileContent(t, used[1])

		test.Equal(t, []string{"TestDir1_3/TestSimple - 2", "TestDir2_2/TestSimple - 1"}, obsolete)
		test.NoError(t, err)

		// Content of snaps is not changed
		test.Equal(t, mockSnap1, []byte(content1))
		test.Equal(t, mockSnap2, []byte(content2))
	})

	t.Run("should update the obsolete snap files", func(t *testing.T) {
		tests, dir1, dir2 := setupTempExamineFiles(
			t,
			loadMockSnap(t, "mock-snap-1"),
			loadMockSnap(t, "mock-snap-2"),
		)
		used := []string{
			filepath.FromSlash(dir1 + "/test1.snap"),
			filepath.FromSlash(dir2 + "/test2.snap"),
		}

		// removing tests from the map means those tests are no longer used
		delete(tests, "TestDir1_3/TestSimple")
		delete(tests, "TestDir2_1/TestSimple")

		obsolete, err := defaultSnap.examineSnaps(tests, used, "", 1, true, false)
		content1 := test.GetFileContent(t, used[0])
		content2 := test.GetFileContent(t, used[1])

		// !!unsorted
		expected1 := `
[TestDir1_2/TestSimple - 1]
int(10)
string hello world 1 2 1
---

[TestDir1_1/TestSimple - 1]

int(1)

string hello world 1 1 1

---
`
		expected2 := `
[TestDir2_2/TestSimple - 1]
int(1000)
string hello world 2 2 1
---
`

		test.Equal(t, []string{
			"TestDir1_3/TestSimple - 1",
			"TestDir1_3/TestSimple - 2",
			"TestDir2_1/TestSimple - 1",
			"TestDir2_1/TestSimple - 3",
			"TestDir2_1/TestSimple - 2",
		},
			obsolete,
		)
		test.NoError(t, err)

		// Content of snaps is not changed
		test.Equal(t, expected1, content1)
		test.Equal(t, expected2, content2)
	})

	t.Run("should sort all tests", func(t *testing.T) {
		mockSnap1 := loadMockSnap(t, "mock-snap-sort-1")
		mockSnap2 := loadMockSnap(t, "mock-snap-sort-2")
		expectedMockSnap1 := loadMockSnap(t, "mock-snap-sort-1-sorted")
		expectedMockSnap2 := loadMockSnap(t, "mock-snap-sort-2-sorted")
		tests, dir1, dir2 := setupTempExamineFiles(
			t,
			mockSnap1,
			mockSnap2,
		)
		used := []string{
			filepath.FromSlash(dir1 + "/test1.snap"),
			filepath.FromSlash(dir2 + "/test2.snap"),
		}

		obsolete, err := defaultSnap.examineSnaps(tests, used, "", 1, false, true)

		test.NoError(t, err)
		test.Equal(t, 0, len(obsolete))

		content1 := test.GetFileContent(t, filepath.FromSlash(dir1+"/test1.snap"))
		content2 := test.GetFileContent(t, filepath.FromSlash(dir2+"/test2.snap"))

		test.Equal(t, string(expectedMockSnap1), content1)
		test.Equal(t, string(expectedMockSnap2), content2)
	})

	t.Run(
		"should not update file if snaps are already sorted and shouldUpdate=false",
		func(t *testing.T) {
			mockSnap1 := loadMockSnap(t, "mock-snap-sort-1-sorted")
			mockSnap2 := loadMockSnap(t, "mock-snap-sort-2-sorted")
			tests, dir1, dir2 := setupTempExamineFiles(
				t,
				mockSnap1,
				mockSnap2,
			)
			used := []string{
				filepath.FromSlash(dir1 + "/test1.snap"),
				filepath.FromSlash(dir2 + "/test2.snap"),
			}

			// removing tests from the map means those tests are no longer used
			delete(tests, "TestDir1_3/TestSimple")
			delete(tests, "TestDir2_1/TestSimple")

			obsolete, err := defaultSnap.examineSnaps(tests, used, "", 1, false, true)

			test.NoError(t, err)
			test.Equal(t, []string{
				"TestDir1_3/TestSimple - 1",
				"TestDir1_3/TestSimple - 2",
				"TestDir2_1/TestSimple - 1",
				"TestDir2_1/TestSimple - 2",
				"TestDir2_1/TestSimple - 3",
			},
				obsolete,
			)

			content1 := test.GetFileContent(t, filepath.FromSlash(dir1+"/test1.snap"))
			content2 := test.GetFileContent(t, filepath.FromSlash(dir2+"/test2.snap"))

			test.Equal(t, string(mockSnap1), content1)
			test.Equal(t, string(mockSnap2), content2)
		},
	)
}

func TestOccurrences(t *testing.T) {
	t.Run("when count 1", func(t *testing.T) {
		tests := map[string]int{
			"add_%d":      3,
			"subtract_%d": 1,
			"divide_%d":   2,
		}

		expected := map[string]struct{}{
			"add_%d - 1":      {},
			"add_%d - 2":      {},
			"add_%d - 3":      {},
			"subtract_%d - 1": {},
			"divide_%d - 1":   {},
			"divide_%d - 2":   {},
		}

		expectedStandalone := map[string]struct{}{
			"add_1":      {},
			"add_2":      {},
			"add_3":      {},
			"subtract_1": {},
			"divide_1":   {},
			"divide_2":   {},
		}

		test.Equal(t, expected, defaultSnap.occurrences(tests, 1, defaultSnap.snapshotOccurrenceFMT))
		test.Equal(t, expectedStandalone, defaultSnap.occurrences(tests, 1, defaultSnap.standaloneOccurrenceFMT))
	})

	t.Run("when count 3", func(t *testing.T) {
		tests := map[string]int{
			"add_%d":      12,
			"subtract_%d": 3,
			"divide_%d":   9,
		}

		expected := map[string]struct{}{
			"add_%d - 1":      {},
			"add_%d - 2":      {},
			"add_%d - 3":      {},
			"add_%d - 4":      {},
			"subtract_%d - 1": {},
			"divide_%d - 1":   {},
			"divide_%d - 2":   {},
			"divide_%d - 3":   {},
		}

		expectedStandalone := map[string]struct{}{
			"add_1":      {},
			"add_2":      {},
			"add_3":      {},
			"add_4":      {},
			"subtract_1": {},
			"divide_1":   {},
			"divide_2":   {},
			"divide_3":   {},
		}

		test.Equal(t, expected, defaultSnap.occurrences(tests, 3, defaultSnap.snapshotOccurrenceFMT))
		test.Equal(t, expectedStandalone, defaultSnap.occurrences(tests, 3, defaultSnap.standaloneOccurrenceFMT))
	})
}

func TestSummary(t *testing.T) {
	for _, v := range []struct {
		name     string
		snapshot string
	}{
		{
			name:     "should print obsolete file",
			snapshot: defaultSnap.summary([]string{"test0.snap"}, nil, 0, nil, false),
		},
		{
			name: "should print obsolete tests",
			snapshot: defaultSnap.summary(
				nil,
				[]string{"TestMock/should_pass - 1", "TestMock/should_pass - 2"},
				0,
				nil,
				false,
			),
		},
		{
			name:     "should print updated file",
			snapshot: defaultSnap.summary([]string{"test0.snap"}, nil, 0, nil, true),
		},
		{
			name:     "should print updated test",
			snapshot: defaultSnap.summary(nil, []string{"TestMock/should_pass - 1"}, 0, nil, true),
		},
		{
			name:     "should return empty string",
			snapshot: defaultSnap.summary(nil, nil, 0, nil, false),
		},
		{
			name: "should print events",
			snapshot: defaultSnap.summary(nil, nil, 0, map[uint8]int{
				added:   5,
				erred:   100,
				updated: 3,
				passed:  10,
			}, false),
		},
		{
			name:     "should print number of skipped tests",
			snapshot: defaultSnap.summary(nil, nil, 1, nil, true),
		},
		{
			name: "should print all summary",
			snapshot: defaultSnap.summary(
				[]string{"test0.snap"},
				[]string{"TestMock/should_pass - 1"},
				5,
				map[uint8]int{
					added:   5,
					erred:   100,
					updated: 3,
					passed:  10,
				},
				false,
			),
		},
	} {
		// capture v
		v := v
		t.Run(v.name, func(t *testing.T) {
			t.Parallel()

			MatchSnapshot(t, v.snapshot)
		})
	}
}

func TestGetTestID(t *testing.T) {
	testCases := []struct {
		input      string
		expectedID string
		valid      bool
	}{
		{"[Test/something - 10]", "Test/something - 10", true},
		{input: "[Test/something - 100231231dsada]", expectedID: "", valid: false},
		{input: "[Test/something - 100231231 ]", expectedID: "", valid: false},
		{input: "[Test/something -100231231 ]", expectedID: "", valid: false},
		{input: "[Test/something- 100231231]", expectedID: "", valid: false},
		{input: "[Test/something - a ]", expectedID: "", valid: false},
		{"[Test123 - Some Test]", "", false},
		{"", "", false},
		{"Invalid input", "", false},
		{"[Test - Missing Closing Bracket", "", false},
		{"[TesGetTestID- No Space]", "", false},
		// must have [
		{"Test something 10]", "", false},
		// must have Test at the start
		{"TesGetTestID -   ]", "", false},
		// must have dash between test name and number
		{"[Test something 10]", "", false},
		{"[Test/something - not a number]", "", false},
		{"s", "", false},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			// make sure that the capacity of b is len(tc.input), this way
			// indexing beyond the capacity will cause test to panic
			b := make([]byte, 0, len(tc.input))
			b = append(b, []byte(tc.input)...)
			id, ok := defaultSnap.getTestID(b)

			test.Equal(t, tc.valid, ok)
			test.Equal(t, tc.expectedID, id)
		})
	}
}

func TestNaturalSort(t *testing.T) {
	t.Run("should sort in descending order", func(t *testing.T) {
		items := []string{
			"[TestExample/Test_Case_1#74 - 1]",
			"[TestExample/Test_Case_1#05 - 1]",
			"[TestExample/Test_Case_1#09 - 1]",
			"[TestExample - 1]",
			"[TestExample/Test_Case_1#71 - 1]",
			"[TestExample/Test_Case_1#100 - 1]",
			"[TestExample/Test_Case_1#7 - 1]",
		}
		expected := []string{
			"[TestExample - 1]",
			"[TestExample/Test_Case_1#05 - 1]",
			"[TestExample/Test_Case_1#7 - 1]",
			"[TestExample/Test_Case_1#09 - 1]",
			"[TestExample/Test_Case_1#71 - 1]",
			"[TestExample/Test_Case_1#74 - 1]",
			"[TestExample/Test_Case_1#100 - 1]",
		}

		slices.SortFunc(items, defaultSnap.naturalSort)

		test.Equal(t, expected, items)
	})
}

func TestBaseCallerNested(t *testing.T) {
	file := defaultSnap.baseCaller(0)

	test.Contains(t, file, "/snaps/contracts_test.go")
}

func testBaseCallerNested(t *testing.T) {
	file := defaultSnap.baseCaller(0)

	test.Contains(t, file, "/snaps/contracts_test.go")
}

func TestBaseCallerHelper(t *testing.T) {
	t.Helper()
	file := defaultSnap.baseCaller(0)

	test.Contains(t, file, "/snaps/contracts_test.go")
}

func TestBaseCaller(t *testing.T) {
	t.Run("should return correct baseCaller", func(t *testing.T) {
		var file string

		func() {
			file = defaultSnap.baseCaller(1)
		}()

		test.Contains(t, file, "/snaps/contracts_test.go")
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

	t.Run("testSkipped", func(t *testing.T) {
		t.Run("should return true if testID is not part of the 'runOnly'", func(t *testing.T) {
			runOnly := "TestMock"
			testID := "TestSkip/should_call_Skip - 1"

			received := defaultSnap.testSkipped(testID, runOnly)
			test.True(t, received)
		})

		t.Run("should return false if testID is part of 'runOnly'", func(t *testing.T) {
			runOnly := "TestMock"
			testID := "TestMock/Test/should_be_not_skipped - 2"

			received := defaultSnap.testSkipped(testID, runOnly)
			test.False(t, received)
		})

		t.Run(
			"should check if the parent is skipped and mark child tests as skipped",
			func(t *testing.T) {
				t.Cleanup(func() {
					defaultSnap.skippedTests = make([]string, 0)
				})

				runOnly := ""
				mockT := test.NewMockTestingT(t)
				mockT.MockName = func() string {
					return "TestMock/Skip"
				}
				mockT.MockLog = func(args ...any) {
					test.Equal(t, skippedMsg, args[0].(string))
				}
				mockT.MockSkipNow = func() {}

				// This is for populating skippedTests.values and following the normal flow
				SkipNow(mockT)

				test.True(t, defaultSnap.testSkipped("TestMock/Skip - 1000", runOnly))
				test.True(
					t,
					defaultSnap.testSkipped("TestMock/Skip/child_should_also_be_skipped", runOnly),
				)
				test.False(t, defaultSnap.testSkipped("TestAnotherTest", runOnly))
			},
		)

		t.Run("should not mark tests skipped if not not a child", func(t *testing.T) {
			t.Cleanup(func() {
				defaultSnap.skippedTests = make([]string, 0)
			})

			runOnly := ""
			mockT := test.NewMockTestingT(t)
			mockT.MockName = func() string {
				return "Test"
			}
			mockT.MockLog = func(args ...any) {
				test.Equal(t, skippedMsg, args[0].(string))
			}
			mockT.MockSkipNow = func() {}

			// This is for populating skippedTests.values and following the normal flow
			SkipNow(mockT)

			test.True(t, defaultSnap.testSkipped("Test - 1", runOnly))
			test.True(t, defaultSnap.testSkipped("Test/child - 1", runOnly))
			test.False(t, defaultSnap.testSkipped("TestMock - 1", runOnly))
			test.False(t, defaultSnap.testSkipped("TestMock/child - 1", runOnly))
		})

		t.Run("should use regex match for runOnly", func(t *testing.T) {
			test.False(t, defaultSnap.testSkipped("MyTest - 1", "Test"))
			test.True(t, defaultSnap.testSkipped("MyTest - 1", "^Test"))
		})
	})

	t.Run("isFileSkipped", func(t *testing.T) {
		t.Run("should return 'false'", func(t *testing.T) {
			test.False(t, defaultSnap.isFileSkipped("", "", ""))
		})

		t.Run("should return 'true' if test is not included in the test file", func(t *testing.T) {
			dir, _ := os.Getwd()

			test.Equal(
				t,
				true,
				defaultSnap.isFileSkipped(dir+"/__snapshots__", "skip_test.snap", "TestNonExistent"),
			)
		})

		t.Run("should return 'false' if test is included in the test file", func(t *testing.T) {
			dir, _ := os.Getwd()

			test.False(t, defaultSnap.isFileSkipped(dir+"/__snapshots__", "skip_test.snap", "TestSkip"))
		})

		t.Run("should use regex match for runOnly", func(t *testing.T) {
			dir, _ := os.Getwd()

			test.Equal(
				t,
				false,
				defaultSnap.isFileSkipped(dir+"/__snapshots__", "skip_test.snap", "TestSkip.*"),
			)
		})
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
