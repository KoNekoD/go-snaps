package matchers

import (
	"errors"
	"fmt"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type TypeMatcher[ExpectedType any] struct {
	paths            []string
	errOnMissingPath bool
	name             string
	expectedType     any
}

func Type[ExpectedType any](paths ...string) *TypeMatcher[ExpectedType] {
	return &TypeMatcher[ExpectedType]{paths: paths, errOnMissingPath: true, name: "Type", expectedType: *new(ExpectedType)}
}

func (t *TypeMatcher[T]) ErrOnMissingPath(e bool) *TypeMatcher[T] {
	t.errOnMissingPath = e
	return t
}

func (t *TypeMatcher[ExpectedType]) JSON(s []byte) ([]byte, []MatcherError) {
	var errs []MatcherError
	json := s

	for _, path := range t.paths {
		r := gjson.GetBytes(json, path)
		if !r.Exists() {
			if t.errOnMissingPath {
				errs = append(errs, MatcherError{Reason: errors.New("path does not exist"), Matcher: t.name, Path: path})
			}
			continue
		}

		if _, ok := r.Value().(ExpectedType); !ok {
			errs = append(errs, MatcherError{Reason: fmt.Errorf("expected type %T, received %T", *new(ExpectedType), r.Value()), Matcher: t.name, Path: path})

			continue
		}

		j, err := sjson.SetBytesOptions(json, path, fmt.Sprintf("<Type:%T>", r.Value()), &sjson.Options{Optimistic: true, ReplaceInPlace: true})
		if err != nil {
			errs = append(errs, MatcherError{Reason: err, Matcher: t.name, Path: path})

			continue
		}

		json = j
	}

	return json, errs
}
