// Package query is a read-only, ORM-style query engine over slices of
// json-tagged structs. Fields are addressed by dotted json-tag paths
// (e.g. "security_vulnerability.severity") and validated against the struct type
// when a condition is added, so a bad path or op/type mismatch errors
// deterministically regardless of data.
package query

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Op is a comparison operator for a Where condition.
type Op string

const (
	OpEq Op = "eq" // field == value
	OpIn Op = "in" // field is a member of value (a slice)
	OpLt Op = "lt" // field < value (numerics and time.Time)
	OpGt Op = "gt" // field > value (numerics and time.Time)
	OpLe Op = "le" // field <= value (numerics and time.Time)
	OpGe Op = "ge" // field >= value (numerics and time.Time)
)

// Set is an immutable collection of T that queries run against.
type Set[T any] struct {
	data    []T
	present map[string]bool
}

// NewSet builds a set over data. sourceKeys lists the dotted json key paths
// actually present in the source data was decoded from (a child path implies
// its parents); every queried path is validated against it, so querying a key
// the source never had errors instead of silently matching zero values. Pass
// nil only when data was constructed in memory rather than decoded: every
// field of T then exists by construction.
func NewSet[T any](data []T, sourceKeys map[string]bool) Set[T] {
	if sourceKeys == nil {
		sourceKeys = structKeys[T]()
	}
	return Set[T]{data: data, present: sourceKeys}
}

// Query returns a fresh queryset over the set's data.
func (s Set[T]) Query() *Query[T] {
	return &Query[T]{data: s.data, present: s.present, conds: nil, err: nil}
}

// Query is an immutable set of ANDed conditions over a Set's data. Where
// returns a new Query, so callers accumulate with q = q.Where(...). Nothing
// touches the data until a terminal (All, Count, CountBy, CountByNested) runs.
type Query[T any] struct {
	data    []T
	present map[string]bool
	conds   []cond
	err     error
}

// missingKey reports an error when path is not among the source key paths the
// set was built with. Source keys are hierarchical (a child path implies its
// parents), so checking the full path suffices.
func (q *Query[T]) missingKey(path string) error {
	if !q.present[path] {
		return fmt.Errorf("key %q not present in the loaded data", path)
	}
	return nil
}

// Where returns a new Query with an additional condition. The path, op and
// value are validated against T immediately; an invalid condition is carried
// as an error and surfaced by the terminal. The first error wins. The value
// may be an F referencing another field of T, which compares the two fields
// of each entry row-wise.
func (q *Query[T]) Where(path string, op Op, value any) *Query[T] {
	if q.err != nil {
		return q
	}
	c, err := compile[T](path, op, value)
	if err == nil {
		err = q.missingKey(path)
	}
	if f, isF := value.(F); isF && err == nil {
		err = q.missingKey(f.Path)
	}
	next := &Query[T]{data: q.data, present: q.present, conds: nil, err: err}
	next.conds = append(append([]cond{}, q.conds...), c)
	return next
}

func (q *Query[T]) match(item T) bool {
	for i := range q.conds {
		if !q.conds[i].matches(item) {
			return false
		}
	}
	return true
}

// All returns the entries matching every condition.
func (q *Query[T]) All() ([]T, error) {
	if q.err != nil {
		return nil, q.err
	}
	out := []T{}
	for _, item := range q.data {
		if q.match(item) {
			out = append(out, item)
		}
	}
	return out, nil
}

// Count returns the number of matching entries.
func (q *Query[T]) Count() (int, error) {
	if q.err != nil {
		return 0, q.err
	}
	n := 0
	for _, item := range q.data {
		if q.match(item) {
			n++
		}
	}
	return n, nil
}

// Distinct returns the sorted unique values of the string field at path
// across matching entries.
func (q *Query[T]) Distinct(path string) ([]string, error) {
	if q.err != nil {
		return nil, q.err
	}
	key, err := compileKey[T](path)
	if err != nil {
		return nil, err
	}
	if keyErr := q.missingKey(path); keyErr != nil {
		return nil, keyErr
	}
	seen := map[string]bool{}
	out := []string{}
	for _, item := range q.data {
		if !q.match(item) || seen[key(item)] {
			continue
		}
		seen[key(item)] = true
		out = append(out, key(item))
	}
	sort.Strings(out)
	return out, nil
}

// CountBy tallies matching entries grouped by the string field at path.
// keys pre-seed groups so they appear with a zero count even when unmatched.
func (q *Query[T]) CountBy(path string, keys ...string) (GroupCount, error) {
	gc := GroupCount{Groups: map[string]int{}}
	if q.err != nil {
		return gc, q.err
	}
	group, err := compileKey[T](path)
	if err != nil {
		return gc, err
	}
	if keyErr := q.missingKey(path); keyErr != nil {
		return gc, keyErr
	}
	for _, k := range keys {
		gc.Groups[k] = 0
	}
	for _, item := range q.data {
		if !q.match(item) {
			continue
		}
		gc.Groups[group(item)]++
		gc.Total++
	}
	return gc, nil
}

// CountByNested tallies matching entries grouped by the string field at outer,
// then by the string field at inner within each outer group.
func (q *Query[T]) CountByNested(outer, inner string, innerKeys ...string) (map[string]GroupCount, error) {
	if q.err != nil {
		return nil, q.err
	}
	outerKey, err := compileKey[T](outer)
	if err != nil {
		return nil, err
	}
	innerKey, err := compileKey[T](inner)
	if err != nil {
		return nil, err
	}
	if keyErr := q.missingKey(outer); keyErr != nil {
		return nil, keyErr
	}
	if keyErr := q.missingKey(inner); keyErr != nil {
		return nil, keyErr
	}
	out := map[string]GroupCount{}
	for _, item := range q.data {
		if !q.match(item) {
			continue
		}
		gc, ok := out[outerKey(item)]
		if !ok {
			gc = GroupCount{Groups: map[string]int{}}
			for _, k := range innerKeys {
				gc.Groups[k] = 0
			}
		}
		gc.Groups[innerKey(item)]++
		gc.Total++
		out[outerKey(item)] = gc
	}
	return out, nil
}

// GroupCount holds per-group tallies and the total across groups.
type GroupCount struct {
	Groups map[string]int
	Total  int
}

// MarshalJSON emits the groups as a flat object with "total" alongside them:
// {"low":1,"high":2,"total":3}.
func (g GroupCount) MarshalJSON() ([]byte, error) {
	m := make(map[string]int, len(g.Groups)+1)
	for k, v := range g.Groups {
		m[k] = v
	}
	m["total"] = g.Total
	return json.Marshal(m)
}

// Decode unmarshals the group counts into v, for callers that want their own
// typed struct instead of map access.
func (g GroupCount) Decode(v any) error {
	b, err := g.MarshalJSON()
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
