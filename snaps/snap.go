package snaps

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/KoNekoD/diff-color/pkg/diff"
	"github.com/KoNekoD/go-snaps/snaps/colors"
	"github.com/KoNekoD/go-snaps/snaps/matchers"
	"github.com/KoNekoD/go-snaps/snaps/symbols"
	"github.com/gkampitakis/ciinfo"
	"github.com/tidwall/gjson"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

type TestingT interface {
	Helper()
	Skip(...any)
	Skipf(string, ...any)
	SkipNow()
	Name() string
	Error(...any)
	Log(...any)
	Cleanup(func())
}

var (
	defaultSnap     = newSnap(defaultConfig(), nil)
	defaultRegistry = newSnapRegistry()
	isCI            = ciinfo.IsCI
	updateVAR       = os.Getenv("UPDATE_SNAPS")
	skippedMsg      = colors.Sprint(colors.Yellow, symbols.SkipSymbol+"Snapshot skipped")
	addedMsg        = colors.Sprint(colors.Green, symbols.UpdateSymbol+"Snapshot added")
	updatedMsg      = colors.Sprint(colors.Green, symbols.UpdateSymbol+"Snapshot updated")
	errInvalidJSON  = errors.New("invalid json")
	errSnapNotFound = errors.New("snapshot not found")
)

const (
	erred uint8 = iota
	added
	updated
	passed
)

type snapRegistry struct {
	testEvents      map[uint8]int
	testEventsMutex sync.Mutex

	registryRunning map[string]int
	registryCleanup map[string]int
	registryMutex   sync.Mutex
}

func newSnapRegistry() *snapRegistry {
	return &snapRegistry{testEvents: make(map[uint8]int), registryRunning: make(map[string]int), registryCleanup: make(map[string]int)}
}

type snap struct {
	c                  *Config
	t                  TestingT
	fileExtension      string
	skippedTests       []string
	skippedTestsMutex  sync.Mutex
	registry           *snapRegistry
	snapshotSerializer *snapshotSerializer
}

func newSnap(c *Config, t TestingT) *snap {
	return &snap{c: c, t: t, skippedTests: make([]string, 0), registry: defaultRegistry, fileExtension: ".snap", snapshotSerializer: newSnapshotSerializer(c)}
}

func (s *snap) withTesting(t TestingT) *snap {
	s.t = t

	return s
}

func (s *snap) matchStandaloneSnapshot(v any) {
	s.t.Helper()
	s.handleSnapshot(s.snapshotSerializer.takeSnapshot(v))
}

func (s *snap) matchSnapshot(v ...any) {
	s.t.Helper()

	if len(v) == 0 {
		s.t.Log(colors.Sprint(colors.Yellow, "[warning] MatchSnapshot call without params\n"))
		return
	}

	s.handleSnapshot(s.snapshotSerializer.takeSliceSnapshot(v))
}

func (s *snap) matchJson(input any, matchers ...matchers.JsonMatcher) {
	s.fileExtension = ".json"
	s.t.Helper()

	v, err := s.validateJson(input)
	if err != nil {
		s.handleError(err)
		return
	}

	v, matchersErrors := s.applyJsonMatchers(v, matchers...)
	if len(matchersErrors) > 0 {
		sb := strings.Builder{}
		for _, err := range matchersErrors {
			colors.Fprint(&sb, colors.Red, fmt.Sprintf("\n%smatch.%s(\"%s\") - %s", symbols.ErrorSymbol, err.Matcher, err.Path, err.Reason))
		}
		s.handleError(sb.String())
		return
	}

	s.handleSnapshot(s.snapshotSerializer.takeJsonSnapshot(v))
}

func (s *snap) handleSnapshot(actualSerializedSnapshot string) {
	s.t.Helper()
	genericPathSnap, genericSnapPathRel := s.snapshotPath()
	snapPath, snapPathRel := s.getTestIdFromRegistry(genericPathSnap, genericSnapPathRel)
	s.t.Cleanup(func() { s.resetSnapPathInRegistry(genericPathSnap) })

	fileBytes, err := os.ReadFile(snapPath)
	if err != nil {
		if isCI {
			s.handleError(errSnapNotFound)
			return
		}
		err := s.upsertStandaloneSnapshot(actualSerializedSnapshot, snapPath)
		if err != nil {
			s.handleError(err)
			return
		}
		s.t.Log(addedMsg)
		s.registerTestEvent(added)
		return
	}
	savedSerializedSnapshot := string(fileBytes)

	// savedSerializedSnapshot ( Unmarshall and Marshall again )
	var savedSnapshot map[string]interface{}
	if err := json.Unmarshal([]byte(savedSerializedSnapshot), &savedSnapshot); err == nil { // is json's related
		savedSerializedSnapshot = s.snapshotSerializer.takeJsonSnapshot([]byte(savedSerializedSnapshot))
	}

	expected := savedSerializedSnapshot
	received := actualSerializedSnapshot

	successfullyDeserialized := true // json's related
	var savedSnapshotRaw map[string]interface{}
	var actualSnapshotRaw map[string]interface{}
	if err := json.Unmarshal([]byte(savedSerializedSnapshot), &savedSnapshotRaw); err != nil {
		successfullyDeserialized = false
	}
	if err := json.Unmarshal([]byte(actualSerializedSnapshot), &actualSnapshotRaw); err != nil {
		successfullyDeserialized = false
	}

	prettyDiff := ""
	if expected != received && !(reflect.DeepEqual(savedSnapshotRaw, actualSnapshotRaw) && successfullyDeserialized) {
		differ := getUnifiedDiff
		if shouldPrintHighlights(expected, received) {
			differ = singlelineDiff
		}
		finalDiff, i, d := differ(expected, received)
		_ = diff.Diff(expected, received) // TODO: Add possibility to change diff printer, now alternative is disabled
		prettyDiff = buildDiffReport(i, d, finalDiff, snapPathRel, 1)
	}
	if prettyDiff == "" {
		s.registerTestEvent(passed)
		return
	}
	if !s.shouldUpdate() {
		s.handleError(prettyDiff)
		return
	}
	if err = s.upsertStandaloneSnapshot(actualSerializedSnapshot, snapPath); err != nil {
		s.handleError(err)
		return
	}
	s.t.Log(updatedMsg)
	s.registerTestEvent(updated)
}

func (s *snap) snapshotPath() (string, string) {
	s.t.Helper()
	callerFilename := s.baseCaller(4) //  skips current func, the wrapper match* and the exported Match* func
	dir := s.c.SnapsDir()
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(filepath.Dir(callerFilename), s.c.SnapsDir())
	}
	snapPath := filepath.Join(dir, s.constructFilename(callerFilename))
	snapPathRel, _ := filepath.Rel(filepath.Dir(callerFilename), snapPath)

	return snapPath, snapPathRel
}

func (s *snap) constructFilename(callerFilename string) string {
	filename := s.c.Filename()
	if filename == "" {
		base := filepath.Base(callerFilename)
		filename = strings.TrimSuffix(base, filepath.Ext(base))
		filename = strings.ReplaceAll(s.t.Name(), "/", "_")
	}
	filename += "_%d"
	filename += s.fileExtension + s.c.Extension()

	return filename
}

func (s *snap) applyJsonMatchers(b []byte, matchersList ...matchers.JsonMatcher) ([]byte, []matchers.MatcherError) {
	var matcherErrors []matchers.MatcherError

	for _, m := range matchersList {
		jsonBytes, errs := m.JSON(b)
		if len(errs) > 0 {
			matcherErrors = append(matcherErrors, errs...)
			continue
		}
		b = jsonBytes
	}

	return b, matcherErrors
}

func (s *snap) shouldUpdate() bool {
	if isCI {
		return false
	}
	if u := s.c.Update(); u != nil {
		return *u
	}

	return "true" == updateVAR
}

func (s *snap) validateJson(input any) ([]byte, error) {
	switch j := input.(type) {
	case string:
		if !gjson.Valid(j) {
			return nil, errInvalidJSON
		}

		return []byte(j), nil
	case []byte:
		if !gjson.ValidBytes(j) {
			return nil, errInvalidJSON
		}

		return j, nil
	default:
		return json.Marshal(input)
	}
}

func (s *snap) handleError(err any) {
	s.t.Helper()
	s.t.Error(err)
	s.registerTestEvent(erred)
}

func (s *snap) upsertStandaloneSnapshot(snapshot, snapPath string) error {
	if err := os.MkdirAll(filepath.Dir(snapPath), os.ModePerm); err != nil {
		return err
	}
	return os.WriteFile(snapPath, []byte(snapshot), os.ModePerm)
}

func (s *snap) trackSkip() {
	s.t.Helper()
	s.t.Log(skippedMsg)
	s.skippedTestsMutex.Lock()
	s.skippedTests = append(s.skippedTests, s.t.Name())
	s.skippedTestsMutex.Unlock()
}

func (s *snap) baseCaller(skip int) string {
	var (
		pc             uintptr
		file, prevFile string
		ok             bool
	)

	for i := skip + 1; ; i++ {
		prevFile = file
		pc, file, _, ok = runtime.Caller(i)
		if !ok {
			return prevFile
		}

		f := runtime.FuncForPC(pc)
		if f == nil {
			return prevFile
		}

		if f.Name() == "testing.tRunner" {
			return prevFile
		}

		if strings.HasSuffix(filepath.Base(file), "_test.go") {
			return file
		}
	}
}

func (s *snap) registerTestEvent(event uint8) {
	s.registry.testEventsMutex.Lock()
	defer s.registry.testEventsMutex.Unlock()
	s.registry.testEvents[event]++
}

func (s *snap) getTestIdFromRegistry(snapPath, snapPathRel string) (string, string) {
	s.registry.registryMutex.Lock()
	s.registry.registryRunning[snapPath]++
	s.registry.registryCleanup[snapPath]++
	c := s.registry.registryRunning[snapPath]
	s.registry.registryMutex.Unlock()

	return fmt.Sprintf(snapPath, c), fmt.Sprintf(snapPathRel, c)
}

func (s *snap) resetSnapPathInRegistry(snapPath string) {
	s.registry.registryMutex.Lock()
	s.registry.registryRunning[snapPath] = 0
	s.registry.registryMutex.Unlock()
}
