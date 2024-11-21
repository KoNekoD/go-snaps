package snaps

import (
	valuePretty "github.com/kr/pretty"
	jsonPretty "github.com/tidwall/pretty"
	"strings"
)

type snapshotSerializer struct{}

func newSnapshotSerializer() *snapshotSerializer {
	return &snapshotSerializer{}
}

func (s *snapshotSerializer) takeJsonSnapshot(b []byte) string {
	return strings.TrimSuffix(string(jsonPretty.PrettyOptions(b, &jsonPretty.Options{SortKeys: true, Indent: " "})), "\n")
}

func (s *snapshotSerializer) takeSnapshot(object any) string {
	return valuePretty.Sprint(object)
}

func (s *snapshotSerializer) takeSliceSnapshot(objects []any) string {
	snapshots := make([]string, len(objects))
	for i, object := range objects {
		snapshots[i] = valuePretty.Sprint(object)
	}
	return strings.Join(snapshots, "\n")
}
