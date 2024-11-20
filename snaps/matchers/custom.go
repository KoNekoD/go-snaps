package matchers

import (
	"errors"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type CustomMatcher struct {
	callback         func(val any) (any, error)
	errOnMissingPath bool
	name             string
	path             string
}

type CustomCallback func(val any) (any, error)

func Custom(path string, callback CustomCallback) *CustomMatcher {
	return &CustomMatcher{errOnMissingPath: true, callback: callback, name: "Custom", path: path}
}

func (c *CustomMatcher) ErrOnMissingPath(e bool) *CustomMatcher {
	c.errOnMissingPath = e
	return c
}

func (c *CustomMatcher) JSON(s []byte) ([]byte, []MatcherError) {
	r := gjson.GetBytes(s, c.path)
	if !r.Exists() {
		if c.errOnMissingPath {
			return nil, []MatcherError{{Reason: errors.New("path does not exist"), Matcher: c.name, Path: c.path}}
		}

		return s, nil
	}

	value, err := c.callback(r.Value())
	if err != nil {
		return nil, []MatcherError{{Reason: err, Matcher: c.name, Path: c.path}}
	}

	s, err = sjson.SetBytesOptions(s, c.path, value, &sjson.Options{Optimistic: true, ReplaceInPlace: true})
	if err != nil {
		return nil, []MatcherError{{Reason: err, Matcher: c.name, Path: c.path}}
	}

	return s, nil
}
