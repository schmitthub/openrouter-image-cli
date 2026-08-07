package query

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

var timeType = reflect.TypeOf(time.Time{})

// F references another field of the queried struct by dotted json-tag path,
// for use as the value of a Where condition: the condition then compares the
// two fields of each entry row-wise instead of against a constant. Offset
// shifts the referenced field before comparing and requires time fields.
type F struct {
	Path   string
	Offset time.Duration
}

// cond is one validated condition: a field index chain into T plus a value
// already normalized to the field's type, or a second field index chain when
// the value is an F reference.
type cond struct {
	idx  []int
	op   Op
	want reflect.Value   // eq/lt/gt/le/ge with a literal value
	in   []reflect.Value // in
	fidx []int           // eq/lt/gt/le/ge with an F value: referenced field
	foff time.Duration   // F.Offset
	fto  reflect.Type    // F: type of the field at idx, to convert into
}

// compile validates path, op and value against T's type and returns an
// executable condition.
func compile[T any](path string, op Op, value any) (cond, error) {
	c := cond{op: op}
	idx, ft, err := indexFor(reflect.TypeFor[T](), path)
	if err != nil {
		return c, fmt.Errorf("where %q: %w", path, err)
	}
	c.idx = idx

	if f, isF := value.(F); isF {
		return compileFieldRef[T](c, path, ft, op, f)
	}

	switch op {
	case OpEq:
		c.want, err = normalize(ft, value)
	case OpIn:
		rv := reflect.ValueOf(value)
		if !rv.IsValid() || (rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array) {
			return c, fmt.Errorf("where %q in: value must be a slice, got %T", path, value)
		}
		for i := 0; i < rv.Len(); i++ {
			w, nerr := normalize(ft, rv.Index(i).Interface())
			if nerr != nil {
				err = nerr
				break
			}
			c.in = append(c.in, w)
		}
	case OpLt, OpGt, OpLe, OpGe:
		if !isOrdered(ft) {
			return c, fmt.Errorf("where %q: op %s needs a numeric or time field, got %s", path, op, ft)
		}
		c.want, err = normalize(ft, value)
	default:
		return c, fmt.Errorf("where %q: unknown op %q", path, op)
	}
	if err != nil {
		return c, fmt.Errorf("where %q %s: %w", path, op, err)
	}
	return c, nil
}

// compileFieldRef validates an F-valued condition: the referenced path must
// resolve in T, both field types must be comparable under the same rules as
// literal values, and a non-zero offset requires time fields.
func compileFieldRef[T any](c cond, path string, ft reflect.Type, op Op, f F) (cond, error) {
	if op == OpIn {
		return c, fmt.Errorf("where %q: op in does not accept a field reference", path)
	}
	fidx, rft, err := indexFor(reflect.TypeFor[T](), f.Path)
	if err != nil {
		return c, fmt.Errorf("where %q: field ref: %w", path, err)
	}
	fc, rc := class(ft), class(rft)
	compatible := fc == rc || (isNumeric(fc) && isNumeric(rc))
	if !compatible || fc == classNone {
		return c, fmt.Errorf(
			"where %q: field type %s incompatible with referenced field %q type %s",
			path, ft, f.Path, rft,
		)
	}
	if op != OpEq && !isOrdered(ft) {
		return c, fmt.Errorf("where %q: op %s needs a numeric or time field, got %s", path, op, ft)
	}
	if f.Offset != 0 && fc != classTime {
		return c, fmt.Errorf("where %q: field ref offset needs time fields, got %s", path, ft)
	}
	c.fidx = fidx
	c.foff = f.Offset
	c.fto = ft
	return c, nil
}

// compileKey validates that path resolves to a string field of T and returns
// a group-key extractor. A nil pointer along the path yields "".
func compileKey[T any](path string) (func(T) string, error) {
	idx, ft, err := indexFor(reflect.TypeFor[T](), path)
	if err != nil {
		return nil, fmt.Errorf("field %q: %w", path, err)
	}
	if ft.Kind() != reflect.String {
		return nil, fmt.Errorf("field %q: group field must be a string, got %s", path, ft)
	}
	return func(item T) string {
		fv, ok := fieldByIndex(reflect.ValueOf(item), idx)
		if !ok {
			return ""
		}
		return fv.String()
	}, nil
}

// indexFor resolves a dotted json-tag path against t, one segment per struct
// level, and returns the field index chain and the (pointer-dereferenced)
// leaf type.
func indexFor(t reflect.Type, path string) ([]int, reflect.Type, error) {
	var idx []int
	cur := t
	for _, seg := range strings.Split(path, ".") {
		cur = derefType(cur)
		if cur.Kind() != reflect.Struct || cur == timeType {
			return nil, nil, fmt.Errorf("segment %q: %s is not a struct", seg, cur)
		}
		f, ok := fieldByTag(cur, seg)
		if !ok {
			return nil, nil, fmt.Errorf("unknown field %q in %s", seg, cur)
		}
		idx = append(idx, f.Index[0])
		cur = f.Type
	}
	return idx, derefType(cur), nil
}

// structKeys returns the dotted json path of every addressable field of T,
// the source keys for data constructed in memory rather than decoded from
// json. Recursion mirrors indexFor: structs descend, everything else is a
// leaf.
func structKeys[T any]() map[string]bool {
	keys := map[string]bool{}
	collectStructKeys(derefType(reflect.TypeFor[T]()), "", keys)
	return keys
}

func collectStructKeys(t reflect.Type, prefix string, keys map[string]bool) {
	for f := range t.Fields() {
		tag, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if tag == "" || tag == "-" {
			continue
		}
		path := tag
		if prefix != "" {
			path = prefix + "." + tag
		}
		keys[path] = true
		ft := derefType(f.Type)
		if ft.Kind() == reflect.Struct && ft != timeType {
			collectStructKeys(ft, path, keys)
		}
	}
}

func fieldByTag(t reflect.Type, name string) (reflect.StructField, bool) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if tag == name {
			return f, true
		}
	}
	return reflect.StructField{}, false
}

// fieldByIndex walks v down idx, dereferencing pointers. ok is false when a
// nil pointer is hit.
func fieldByIndex(v reflect.Value, idx []int) (reflect.Value, bool) {
	for _, i := range idx {
		for v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return reflect.Value{}, false
			}
			v = v.Elem()
		}
		v = v.Field(i)
	}
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return reflect.Value{}, false
		}
		v = v.Elem()
	}
	return v, true
}

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

// normalize converts value to the field type ft, requiring both sides to be
// the same class (string, bool, numeric, or time) so reflect's cross-class
// conversions (e.g. int→string) can't slip through.
func normalize(ft reflect.Type, value any) (reflect.Value, error) {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return reflect.Value{}, fmt.Errorf("value is nil")
	}
	fc, vc := class(ft), class(rv.Type())
	compatible := fc == vc || (isNumeric(fc) && isNumeric(vc))
	if !compatible || fc == classNone {
		return reflect.Value{}, fmt.Errorf("field type %s incompatible with value type %T", ft, value)
	}
	return rv.Convert(ft), nil
}

type fieldClass int

const (
	classNone fieldClass = iota
	classString
	classBool
	classInt
	classUint
	classFloat
	classTime
)

func class(t reflect.Type) fieldClass {
	if t == timeType {
		return classTime
	}
	switch t.Kind() {
	case reflect.String:
		return classString
	case reflect.Bool:
		return classBool
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return classInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return classUint
	case reflect.Float32, reflect.Float64:
		return classFloat
	default:
		return classNone
	}
}

func isNumeric(c fieldClass) bool {
	return c == classInt || c == classUint || c == classFloat
}

func isOrdered(t reflect.Type) bool {
	c := class(t)
	return isNumeric(c) || c == classTime
}

func (c *cond) matches(item any) bool {
	fv, ok := fieldByIndex(reflect.ValueOf(item), c.idx)
	if !ok {
		return false // nil pointer along the path never matches
	}
	want := c.want
	if c.fidx != nil {
		rv, refOK := fieldByIndex(reflect.ValueOf(item), c.fidx)
		if !refOK {
			return false
		}
		if c.foff != 0 {
			// compile only allows a non-zero offset on time fields
			if tv, isTime := rv.Interface().(time.Time); isTime {
				rv = reflect.ValueOf(tv.Add(c.foff))
			}
		}
		want = rv.Convert(c.fto)
	}
	switch c.op {
	case OpEq:
		return equal(fv, want)
	case OpIn:
		for _, w := range c.in {
			if equal(fv, w) {
				return true
			}
		}
		return false
	case OpLt:
		return compare(fv, want) < 0
	case OpGt:
		return compare(fv, want) > 0
	case OpLe:
		return compare(fv, want) <= 0
	case OpGe:
		return compare(fv, want) >= 0
	}
	return false
}

func equal(a, b reflect.Value) bool {
	if a.Type() == timeType {
		return compare(a, b) == 0
	}
	return a.Equal(b)
}

// compare returns -1/0/1 for two values of the same, ordered type.
func compare(a, b reflect.Value) int {
	switch class(a.Type()) {
	case classTime:
		at, bt := a.Interface().(time.Time), b.Interface().(time.Time)
		return at.Compare(bt)
	case classInt:
		return cmpOrdered(a.Int(), b.Int())
	case classUint:
		return cmpOrdered(a.Uint(), b.Uint())
	case classFloat:
		return cmpOrdered(a.Float(), b.Float())
	}
	return 0
}

func cmpOrdered[T int64 | uint64 | float64](a, b T) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
