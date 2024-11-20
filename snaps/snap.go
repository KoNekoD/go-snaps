package snaps

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gkampitakis/ciinfo"
	"github.com/gkampitakis/go-snaps/snaps/colors"
	"github.com/gkampitakis/go-snaps/snaps/matchers"
	"github.com/gkampitakis/go-snaps/snaps/symbols"
	valuePretty "github.com/kr/pretty"
	"github.com/maruel/natural"
	"github.com/tidwall/gjson"
	jsonPretty "github.com/tidwall/pretty"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
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
	defaultSnap          = newSnap(defaultConfig(), nil)
	defaultRegistry      = newSnapRegistry()
	isCI                 = ciinfo.IsCI
	updateVAR            = os.Getenv("UPDATE_SNAPS")
	shouldClean          = updateVAR == "true" || updateVAR == "clean"
	jsonOptions          = &jsonPretty.Options{SortKeys: true, Indent: " "}
	endSequenceByteSlice = []byte(endSequence)
	skippedMsg           = colors.Sprint(colors.Yellow, symbols.SkipSymbol+"Snapshot skipped")
	addedMsg             = colors.Sprint(colors.Green, symbols.UpdateSymbol+"Snapshot added")
	updatedMsg           = colors.Sprint(colors.Green, symbols.UpdateSymbol+"Snapshot updated")
	errInvalidJSON       = errors.New("invalid json")
	errSnapNotFound      = errors.New("snapshot not found")
)

const (
	endSequence = "---"
	snapsExt    = ".snap"
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
	c *Config
	t TestingT

	skippedTests      []string
	skippedTestsMutex sync.Mutex

	registry *snapRegistry
}

func newSnap(c *Config, t TestingT) *snap {
	return &snap{c: c, t: t, skippedTests: make([]string, 0), registry: defaultRegistry}
}

func (s *snap) WithTesting(t TestingT) *snap {
	s.t = t

	return s
}

func (s *snap) WithRegistry(r *snapRegistry) *snap {
	s.registry = r

	return s
}

func (s *snap) matchStandaloneSnapshot(value any) {
	s.t.Helper()

	genericPathSnap, genericSnapPathRel := s.snapshotPath()
	snapPath, snapPathRel := s.getTestIdFromRegistry(genericPathSnap, genericSnapPathRel)
	s.t.Cleanup(func() { s.resetSnapPathInRegistry(genericPathSnap) })

	snapshot := valuePretty.Sprint(value)
	prevSnapshot, err := s.getPrevStandaloneSnapshot(snapPath)
	if errors.Is(err, errSnapNotFound) {
		if isCI {
			s.handleError(err)
			return
		}

		err := s.upsertStandaloneSnapshot(snapshot, snapPath)
		if err != nil {
			s.handleError(err)
			return
		}

		s.t.Log(addedMsg)
		s.registerTestEvent(added)
		return
	}
	if err != nil {
		s.handleError(err)
		return
	}

	prettyDiff := PrettyDiff(prevSnapshot, snapshot, snapPathRel, 1)
	if prettyDiff == "" {
		s.registerTestEvent(passed)
		return
	}

	if !s.shouldUpdate() {
		s.handleError(prettyDiff)
		return
	}

	if err = s.upsertStandaloneSnapshot(snapshot, snapPath); err != nil {
		s.handleError(err)
		return
	}

	s.t.Log(updatedMsg)
	s.registerTestEvent(updated)
}

func (s *snap) matchSnapshot(values ...any) {
	s.t.Helper()

	if len(values) == 0 {
		s.t.Log(colors.Sprint(colors.Yellow, "[warning] MatchSnapshot call without params\n"))
		return
	}

	genericPathSnap, genericSnapPathRel := s.snapshotPath()
	snapPath, snapPathRel := s.getTestIdFromRegistry(genericPathSnap, genericSnapPathRel)

	s.t.Cleanup(func() { s.resetSnapPathInRegistry(genericPathSnap) })

	snapshot := s.takeSnapshot(values)
	prevSnapshot, err := s.getPrevStandaloneSnapshot(snapPath)
	if errors.Is(err, errSnapNotFound) {
		if isCI {
			s.handleError(err)
			return
		}

		err := s.upsertStandaloneSnapshot(snapshot, snapPath)
		if err != nil {
			s.handleError(err)
			return
		}

		s.t.Log(addedMsg)
		s.registerTestEvent(added)
		return
	}
	if err != nil {
		s.handleError(err)
		return
	}

	prettyDiff := PrettyDiff(s.unescapeEndChars(prevSnapshot), s.unescapeEndChars(snapshot), snapPathRel, 1)
	if prettyDiff == "" {
		s.registerTestEvent(passed)
		return
	}

	if !s.shouldUpdate() {
		s.handleError(prettyDiff)
		return
	}

	if err = s.upsertStandaloneSnapshot(snapshot, snapPath); err != nil {
		s.handleError(err)
		return
	}

	s.t.Log(updatedMsg)
	s.registerTestEvent(updated)
}

func (s *snap) matchJson(input any, matchers ...matchers.JsonMatcher) {
	s.t.Helper()

	genericPathSnap, genericSnapPathRel := s.snapshotPath()
	snapPath, snapPathRel := s.getTestIdFromRegistry(genericPathSnap, genericSnapPathRel)
	s.t.Cleanup(func() { s.resetSnapPathInRegistry(genericPathSnap) })

	j, err := s.validateJson(input)
	if err != nil {
		s.handleError(err)
		return
	}

	j, matchersErrors := s.applyJsonMatchers(j, matchers...)
	if len(matchersErrors) > 0 {
		sb := strings.Builder{}

		for _, err := range matchersErrors {
			str := fmt.Sprintf("\n%smatch.%s(\"%s\") - %s", symbols.ErrorSymbol, err.Matcher, err.Path, err.Reason)

			colors.Fprint(&sb, colors.Red, str)
		}

		s.handleError(sb.String())
		return
	}

	snapshot := s.takeJsonSnapshot(j)
	prevSnapshot, err := s.getPrevStandaloneSnapshot(snapPath)
	if errors.Is(err, errSnapNotFound) {
		if isCI {
			s.handleError(err)
			return
		}

		err := s.upsertStandaloneSnapshot(snapshot, snapPath)
		if err != nil {
			s.handleError(err)
			return
		}

		s.t.Log(addedMsg)
		s.registerTestEvent(added)
		return
	}
	if err != nil {
		s.handleError(err)
		return
	}

	prettyDiff := PrettyDiff(prevSnapshot, snapshot, snapPathRel, 1)
	if prettyDiff == "" {
		s.registerTestEvent(passed)
		return
	}

	if !s.shouldUpdate() {
		s.handleError(prettyDiff)
		return
	}

	if err = s.upsertStandaloneSnapshot(snapshot, snapPath); err != nil {
		s.handleError(err)
		return
	}

	s.t.Log(updatedMsg)
	s.registerTestEvent(updated)
}

func (s *snap) unescapeEndChars(inputString string) string {
	ss := strings.Split(inputString, "\n")
	for idx, s := range ss {
		if s == "/-/-/-/" {
			ss[idx] = endSequence
		}
	}
	return strings.Join(ss, "\n")
}

func (s *snap) escapeEndChars(inputString string) string {
	ss := strings.Split(inputString, "\n")
	for idx, s := range ss {
		if s == endSequence {
			ss[idx] = "/-/-/-/"
		}
	}
	return strings.Join(ss, "\n")
}

func (s *snap) takeSnapshot(objects []any) string {
	snapshots := make([]string, len(objects))

	for i, object := range objects {
		snapshots[i] = valuePretty.Sprint(object)
	}

	return s.escapeEndChars(strings.Join(snapshots, "\n"))
}

func (s *snap) snapshotPath() (string, string) {
	//  skips current func, the wrapper match* and the exported Match* func
	callerFilename := s.baseCaller(3)

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
	filename += snapsExt + s.c.Extension()

	return filename
}

func (s *snap) takeJsonSnapshot(b []byte) string {
	return strings.TrimSuffix(string(jsonPretty.PrettyOptions(b, jsonOptions)), "\n")
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

	configUpdate := s.c.Update()
	if configUpdate != nil {
		return *configUpdate
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

func (s *snap) getPrevStandaloneSnapshot(snapPath string) (string, error) {
	f, err := os.ReadFile(snapPath)
	if err != nil {
		return "", errSnapNotFound
	}

	return string(f), nil
}

func (s *snap) getSkippedTests() []string {
	return s.skippedTests
}

func (s *snap) trackSkip() {
	s.t.Helper()

	s.t.Log(skippedMsg)
	s.addSkippedTest(s.t.Name())
}

func (s *snap) testSkipped(testID, runOnly string) bool {
	// testID form: Test.*/runName - 1
	testName := strings.Split(testID, " - ")[0]

	for _, name := range s.skippedTests {
		if testName == name || strings.HasPrefix(testName, name+"/") {
			return true
		}
	}

	matched, _ := regexp.MatchString(runOnly, testID)
	return !matched
}

func (s *snap) isFileSkipped(dir, filename, runOnly string) bool {
	// When a file is skipped through CLI with -run flag we can track it
	if runOnly == "" {
		return false
	}

	testFilePath := path.Join(dir, "..", strings.TrimSuffix(filename, snapsExt)+".go")

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, testFilePath, nil, parser.ParseComments)
	if err != nil {
		return false
	}

	for _, decls := range file.Decls {
		funcDecl, ok := decls.(*ast.FuncDecl)
		if !ok {
			continue
		}

		// If the TestFunction is inside the file then it's not skipped
		matched, _ := regexp.MatchString(runOnly, funcDecl.Name.String())
		if matched {
			return false
		}
	}

	return true
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

func (s *snap) addSkippedTest(elems ...string) {
	s.skippedTestsMutex.Lock()
	defer s.skippedTestsMutex.Unlock()

	s.skippedTests = append(s.skippedTests, elems...)
}

func (s *snap) examineFiles(registry map[string]int, registeredStandaloneTests map[string]struct{}, runOnly string, shouldUpdate bool) (obsolete, used []string) {
	uniqueDirs := make(map[string]struct{})

	for snapPaths := range registry {
		uniqueDirs[filepath.Dir(snapPaths)] = struct{}{}
	}

	for snapPaths := range registeredStandaloneTests {
		uniqueDirs[filepath.Dir(snapPaths)] = struct{}{}
	}

	for dir := range uniqueDirs {
		dirContents, _ := os.ReadDir(dir)

		for _, content := range dirContents {
			// this is a sanity check shouldn't have dirs inside the snapshot dirs
			// and only delete any `.snap` files
			if content.IsDir() || !strings.Contains(content.Name(), snapsExt) {
				continue
			}

			snapPath := filepath.Join(dir, content.Name())
			if _, called := registry[snapPath]; called {
				used = append(used, snapPath)
				continue
			}

			// if it's a standalone snapshot we don't add it to used list
			// as we don't need it for the next step, to examine individual snaps inside the file
			// as it contains only one
			if _, ok := registeredStandaloneTests[snapPath]; ok {
				continue
			}

			if s.isFileSkipped(dir, content.Name(), runOnly) {
				continue
			}

			obsolete = append(obsolete, snapPath)

			if !shouldUpdate {
				continue
			}

			if err := os.Remove(snapPath); err != nil {
				fmt.Println(err)
			}
		}
	}

	return obsolete, used
}

func (s *snap) examineSnaps(registry map[string]int, used []string, runOnly string, count int, update, sort bool) ([]string, error) {
	obsoleteTests := make([]string, 0)
	tests := make(map[string]string)
	data := bytes.Buffer{}
	testIDs := make([]string, 0)

	for _, snapPath := range used {
		f, err := os.OpenFile(snapPath, os.O_RDWR, os.ModePerm)
		if err != nil {
			return nil, err
		}

		var hasDiffs bool

		registeredTests := s.occurrences(registry, count, s.snapshotOccurrenceFMT)
		snapshotScanner := s.snapshotScanner(f)

		for snapshotScanner.Scan() {
			b := snapshotScanner.Bytes()
			// Check if line is a test id
			testID, match := s.getTestID(b)
			if !match {
				continue
			}
			testIDs = append(testIDs, testID)

			_, hasRegisteredTests := registeredTests[testID]
			if !hasRegisteredTests && !s.testSkipped(testID, runOnly) {
				obsoleteTests = append(obsoleteTests, testID)
				hasDiffs = true

				s.removeSnapshot(snapshotScanner)
				continue
			}

			for snapshotScanner.Scan() {
				line := snapshotScanner.Bytes()

				if bytes.Equal(line, endSequenceByteSlice) {
					tests[testID] = data.String()

					data.Reset()
					break
				}

				data.Write(line)
				data.WriteByte('\n')
			}
		}

		if err := snapshotScanner.Err(); err != nil {
			return nil, err
		}

		shouldSort := sort && !slices.IsSortedFunc(testIDs, s.naturalSort)
		shouldUpdate := update && hasDiffs

		// if we don't have to "write" anything on the snap we skip
		if !shouldUpdate && !shouldSort {
			_ = f.Close()

			clear(tests)
			testIDs = testIDs[:0]
			data.Reset()

			continue
		}

		if shouldSort {
			// sort testIDs
			slices.SortFunc(testIDs, s.naturalSort)
		}

		if err := s.overwriteFile(f, nil); err != nil {
			return nil, err
		}

		for _, id := range testIDs {
			test, ok := tests[id]
			if !ok {
				continue
			}

			_, _ = fmt.Fprintf(f, "\n[%s]\n%s%s\n", id, test, endSequence)
		}
		_ = f.Close()

		clear(tests)
		testIDs = testIDs[:0]
		data.Reset()
	}

	return obsoleteTests, nil
}

func (s *snap) getTestID(b []byte) (string, bool) {
	if len(b) == 0 {
		return "", false
	}

	// needs to start with [Test and end with ]
	if !bytes.HasPrefix(b, []byte("[Test")) || b[len(b)-1] != ']' {
		return "", false
	}

	// needs to contain ' - '
	separator := bytes.Index(b, []byte(" - "))
	if separator == -1 {
		return "", false
	}

	// needs to have a number after the separator
	if !s.isNumber(b[separator+3 : len(b)-1]) {
		return "", false
	}

	return string(b[1 : len(b)-1]), true
}

func (s *snap) summary(obsoleteFiles, obsoleteTests []string, noSkippedTests int, testEvents map[uint8]int, shouldUpdate bool) string {
	if len(obsoleteFiles) == 0 &&
		len(obsoleteTests) == 0 &&
		len(testEvents) == 0 &&
		noSkippedTests == 0 {
		return ""
	}

	var builder strings.Builder

	objectSummaryList := func(objects []string, name string) {
		subject := name
		action := "obsolete"
		color := colors.Yellow
		if len(objects) > 1 {
			subject = name + "s"
		}
		if shouldUpdate {
			action = "removed"
			color = colors.Green
		}

		colors.Fprint(
			&builder,
			color,
			fmt.Sprintf("\n%s%d snapshot %s %s\n", symbols.ArrowSymbol, len(objects), subject, action),
		)

		for _, object := range objects {
			colors.Fprint(
				&builder,
				colors.Dim,
				fmt.Sprintf("  %s %s%s\n", symbols.EnterSymbol, symbols.BulletSymbol, object),
			)
		}
	}

	_, _ = fmt.Fprintf(&builder, "\n%s\n\n", colors.Sprint(colors.BoldWhite, "Snapshot Summary"))

	s.printEvent(&builder, colors.Green, symbols.SuccessSymbol, "passed", testEvents[passed])
	s.printEvent(&builder, colors.Red, symbols.ErrorSymbol, "failed", testEvents[erred])
	s.printEvent(&builder, colors.Green, symbols.UpdateSymbol, "added", testEvents[added])
	s.printEvent(&builder, colors.Green, symbols.UpdateSymbol, "updated", testEvents[updated])
	s.printEvent(&builder, colors.Yellow, symbols.SkipSymbol, "skipped", noSkippedTests)

	if len(obsoleteFiles) > 0 {
		objectSummaryList(obsoleteFiles, "file")
	}

	if len(obsoleteTests) > 0 {
		objectSummaryList(obsoleteTests, "test")
	}

	if !shouldUpdate && len(obsoleteFiles)+len(obsoleteTests) > 0 {
		it := "it"

		if len(obsoleteFiles)+len(obsoleteTests) > 1 {
			it = "them"
		}

		colors.Fprint(
			&builder,
			colors.Dim,
			fmt.Sprintf(
				"\nTo remove %s, re-run tests with `UPDATE_SNAPS=clean go test ./...`\n",
				it,
			),
		)
	}

	return builder.String()
}

func (s *snap) isNumber(b []byte) bool {
	for i := 0; i < len(b); i++ {
		if b[i] < '0' || b[i] > '9' {
			return false
		}
	}

	return true
}

func (s *snap) printEvent(w io.Writer, color, symbol, verb string, events int) {
	if events == 0 {
		return
	}
	subject := "snapshot"
	if events > 1 {
		subject += "s"
	}

	colors.Fprint(w, color, fmt.Sprintf("%s%v %s %s\n", symbol, events, subject, verb))
}

func (s *snap) standaloneOccurrenceFMT(template string, i int) string {
	return fmt.Sprintf(template, i)
}

func (s *snap) snapshotOccurrenceFMT(template string, i int) string {
	return fmt.Sprintf("%s - %d", template, i)
}

func (s *snap) occurrences(tests map[string]int, count int, formatter func(string, int) string) map[string]struct{} {
	result := make(map[string]struct{}, len(tests))
	for testID, counter := range tests {
		// divide a test's counter by count (how many times the go test suite ran)
		// this gives us how many snapshots were created in a single test run.
		counter = counter / count
		if counter > 1 {
			for i := 1; i <= counter; i++ {
				result[formatter(testID, i)] = struct{}{}
			}
		}
		result[formatter(testID, counter)] = struct{}{}
	}

	return result
}

func (s *snap) naturalSort(a, b string) int {
	if a == b {
		return 0
	}
	if natural.Less(a, b) {
		return -1
	}
	return 1
}

func (s *snap) registerTestEvent(event uint8) {
	s.registry.testEventsMutex.Lock()
	defer s.registry.testEventsMutex.Unlock()
	s.registry.testEvents[event]++
}

func (s *snap) overwriteFile(f *os.File, b []byte) error {
	_ = f.Truncate(0)
	_, _ = f.Seek(0, io.SeekStart)
	_, err := f.Write(b)
	return err
}

func (s *snap) removeSnapshot(scanner *bufio.Scanner) {
	for scanner.Scan() {
		// skip until ---
		if bytes.Equal(scanner.Bytes(), endSequenceByteSlice) {
			break
		}
	}
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

func (s *snap) snapshotScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer([]byte{}, math.MaxInt)
	return scanner
}
