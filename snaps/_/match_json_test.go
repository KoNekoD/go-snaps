package _

import (
	"errors"
	"github.com/gkampitakis/go-snaps/snaps"
	"testing"
)

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
				snaps.setupSnapshot(t, jsonFilename, false)

				mockT := snaps.NewMockTestingT(t)
				mockT.MockError = func(args ...any) {
					snaps.Equal(t, tc.err, (args[0].(error)).Error())
				}

				snaps.MatchJSON(mockT, tc.input)
			})
		}
	})

	t.Run("matchers", func(t *testing.T) {
		t.Run("should apply matchers in order", func(t *testing.T) {
			snaps.setupSnapshot(t, jsonFilename, false)

			mockT := snaps.NewMockTestingT(t)
			mockT.MockLog = func(args ...any) { snaps.Equal(t, snaps.addedMsg, args[0].(string)) }

			c1 := func(val any) (any, error) {
				return map[string]any{"key2": nil}, nil
			}
			c2 := func(val any) (any, error) {
				return map[string]any{"key3": nil}, nil
			}
			c3 := func(val any) (any, error) {
				return map[string]any{"key4": nil}, nil
			}

			snaps.MatchJSON(
				mockT,
				`{"key1":""}`,
				snaps.Custom("key1", c1),
				snaps.Custom("key1.key2", c2),
				snaps.Custom("key1.key2.key3", c3),
			)
		})

		t.Run("should aggregate errors from matchers", func(t *testing.T) {
			snaps.setupSnapshot(t, jsonFilename, false)

			mockT := snaps.NewMockTestingT(t)
			mockT.MockError = func(args ...any) {
				snaps.Equal(t,
					"\x1b[31;1m\n✕ match.Custom(\"age\") - mock error"+
						"\x1b[0m\x1b[31;1m\n✕ match.Any(\"missing.key.1\") - path does not exist"+
						"\x1b[0m\x1b[31;1m\n✕ match.Any(\"missing.key.2\") - path does not exist\x1b[0m",
					args[0],
				)
			}

			c := func(val any) (any, error) {
				return nil, errors.New("mock error")
			}
			snaps.MatchJSON(
				mockT,
				`{"age":10}`,
				snaps.Custom("age", c),
				snaps.Any("missing.key.1", "missing.key.2"),
			)
		})
	})

	t.Run("if it's running on ci should skip creating snapshot", func(t *testing.T) {
		snaps.setupSnapshot(t, jsonFilename, true)

		mockT := snaps.NewMockTestingT(t)
		mockT.MockName = func() string {
			return "mock-name-check-file-not-found"
		}
		mockT.MockError = func(args ...any) {
			snaps.Equal(t, snaps.errSnapNotFound, args[0].(error))
		}

		snaps.MatchJSON(mockT, "{}")

		snaps.Equal(t, 1, snaps.testEvents.items[snaps.erred])
	})

	t.Run("should update snapshot when 'shouldUpdate'", func(t *testing.T) {
		snapPath := snaps.setupSnapshot(t, jsonFilename, false, true)

		printerExpectedCalls := []func(received any){
			func(received any) { snaps.Equal(t, snaps.addedMsg, received.(string)) },
			func(received any) { snaps.Equal(t, snaps.updatedMsg, received.(string)) },
		}
		mockT := snaps.NewMockTestingT(t)
		mockT.MockName = func() string {
			return "mock-name-should-update-when-should-update"
		}
		mockT.MockLog = func(args ...any) {
			printerExpectedCalls[0](args[0])

			// shift
			printerExpectedCalls = printerExpectedCalls[1:]
		}

		// First call for creating the snapshot
		snaps.MatchJSON(mockT, "{\"value\":\"hello world\"}")
		snaps.Equal(t, 1, snaps.testEvents.items[snaps.added])

		// Resetting registry to emulate the same MatchSnapshot call
		snaps.registry = snaps.newStandaloneRegistry()

		// Second call with different params
		snaps.MatchJSON(mockT, "{\"value\":\"bye world\"}")

		snaps.Equal(
			t,
			"\n[mock-name - 1]\n{\n \"value\": \"bye world\"\n}\n---\n",
			snaps.GetFileContent(t, snapPath),
		)
		snaps.Equal(t, 1, snaps.testEvents.items[snaps.updated])
	})
}
