package matchers

import (
	"errors"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type AnyMatcher struct {
	paths            []string
	placeholder      any
	errOnMissingPath bool
	name             string
}

func Any(paths ...string) *AnyMatcher {
	return &AnyMatcher{errOnMissingPath: true, placeholder: "<Any value>", paths: paths, name: "Any"}
}

func (a *AnyMatcher) Placeholder(p any) *AnyMatcher {
	a.placeholder = p
	return a
}

func (a *AnyMatcher) ErrOnMissingPath(e bool) *AnyMatcher {
	a.errOnMissingPath = e
	return a
}

func (a *AnyMatcher) JSON(s []byte) ([]byte, []MatcherError) {
	var errs []MatcherError

	json := s
	for _, path := range a.paths {
		r := gjson.GetBytes(json, path)
		if !r.Exists() {
			if a.errOnMissingPath {
				errs = append(errs, MatcherError{Reason: errors.New("path does not exist"), Matcher: a.name, Path: path})
			}
			continue
		}

		j, err := sjson.SetBytesOptions(json, path, a.placeholder, &sjson.Options{Optimistic: true, ReplaceInPlace: true})
		if err != nil {
			errs = append(errs, MatcherError{Reason: err, Matcher: a.name, Path: path})

			continue
		}

		json = j
	}

	return json, errs
}
