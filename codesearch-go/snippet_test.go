package main

import (
	"reflect"
	"testing"
)

// Tahap 2 maintainability (2026-10-31): the camelCase token sequence must
// stay identical across the three places that compute it, or search,
// ranking and highlighting silently diverge:
//   - index: tokenize(splitCamel(text))        (index.go addChunkDoc)
//   - query: tokenizeQuery(splitCamel(text))   (search.go parseQuery path)
//   - highlight: fineSpans(text) tok fields   (snippet.go snippetWindow)
//
// This test locks that symmetry for a corpus of identifier shapes,
// including acronym runs (URLValue, XMLHttpRequest) where the boundary
// rule (upper followed by lower) is easy to get wrong in one copy.
func TestCamelSymmetry(t *testing.T) {
	cases := []string{
		"getGraphParametersFlat",
		"URLValue",
		"XMLHttpRequest",
		"fooBar123Baz",
		"alreadyLower",
		"fooBARBaz",
		"get2ndValue",
		"snake_case",
		"helloWorld testCase",
		"a",
		"AB",
	}
	for _, s := range cases {
		indexToks := tokenize(splitCamel(s))

		var queryToks []string
		for _, qt := range tokenizeQuery(splitCamel(s)) {
			queryToks = append(queryToks, qt.term)
		}

		var highlightToks []string
		for _, sp := range fineSpans(s) {
			highlightToks = append(highlightToks, sp.tok)
		}

		if !reflect.DeepEqual(indexToks, queryToks) {
			t.Errorf("query drift for %q: index %v, query %v", s, indexToks, queryToks)
		}
		if !reflect.DeepEqual(indexToks, highlightToks) {
			t.Errorf("highlight drift for %q: index %v, highlight %v", s, indexToks, highlightToks)
		}
	}
}
