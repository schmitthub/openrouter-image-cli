package query_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/schmitthub/openrouter-generate/internal/query"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type entry struct {
	Number    int        `json:"number"`
	State     string     `json:"state"`
	Score     float64    `json:"score"`
	CreatedAt time.Time  `json:"created_at"`
	FixedAt   *time.Time `json:"fixed_at"`
	Scope     *string    `json:"scope"`
	Repo      struct {
		FullName string `json:"full_name"`
		Owner    struct {
			ID int `json:"id"`
		} `json:"owner"`
	} `json:"repo"`
}

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func testSet() query.Set[entry] {
	runtime := "runtime"
	fixed := ts("2026-03-01T00:00:00Z")

	e1 := entry{Number: 1, State: "open", Score: 9.8, CreatedAt: ts("2026-01-01T00:00:00Z")}
	e1.Repo.FullName = "org/a"
	e1.Repo.Owner.ID = 10

	e2 := entry{Number: 2, State: "open", Score: 3.1, CreatedAt: ts("2026-02-01T00:00:00Z"), Scope: &runtime}
	e2.Repo.FullName = "org/a"
	e2.Repo.Owner.ID = 10

	e3 := entry{Number: 3, State: "fixed", Score: 7.5, CreatedAt: ts("2026-06-01T00:00:00Z"), FixedAt: &fixed}
	e3.Repo.FullName = "org/b"
	e3.Repo.Owner.ID = 20

	return query.NewSet([]entry{e1, e2, e3}, nil)
}

func numbers(items []entry) []int {
	out := []int{}
	for _, e := range items {
		out = append(out, e.Number)
	}
	return out
}

func TestQuery_All(t *testing.T) {
	s := testSet()

	tests := []struct {
		name string
		q    *query.Query[entry]
		want []int
	}{
		{"no conditions", s.Query(), []int{1, 2, 3}},
		{"eq string", s.Query().Where("state", query.OpEq, "open"), []int{1, 2}},
		{"eq nested path", s.Query().Where("repo.full_name", query.OpEq, "org/b"), []int{3}},
		{"eq deep int", s.Query().Where("repo.owner.id", query.OpEq, 20), []int{3}},
		{"eq pointer string", s.Query().Where("scope", query.OpEq, "runtime"), []int{2}},
		{"in", s.Query().Where("state", query.OpIn, []string{"open", "fixed"}), []int{1, 2, 3}},
		{"lt float", s.Query().Where("score", query.OpLt, 7.5), []int{2}},
		{"gt float", s.Query().Where("score", query.OpGt, 7.5), []int{1}},
		{"lt int accepted for float field", s.Query().Where("score", query.OpLt, 8), []int{2, 3}},
		{"lt time", s.Query().Where("created_at", query.OpLt, ts("2026-02-15T00:00:00Z")), []int{1, 2}},
		{"gt time", s.Query().Where("created_at", query.OpGt, ts("2026-02-15T00:00:00Z")), []int{3}},
		{"lt pointer time skips nils", s.Query().Where("fixed_at", query.OpLt, ts("2027-01-01T00:00:00Z")), []int{3}},
		{
			"le time includes boundary",
			s.Query().Where("created_at", query.OpLe, ts("2026-02-01T00:00:00Z")),
			[]int{1, 2},
		},
		{
			"ge time includes boundary",
			s.Query().Where("created_at", query.OpGe, ts("2026-02-01T00:00:00Z")),
			[]int{2, 3},
		},
		{
			"field ref lt time",
			s.Query().Where("fixed_at", query.OpLt, query.F{Path: "created_at", Offset: 0}),
			[]int{3},
		},
		{
			"field ref gt time no match",
			s.Query().Where("fixed_at", query.OpGt, query.F{Path: "created_at", Offset: 0}),
			[]int{},
		},
		{
			"field ref nil pointer skipped",
			s.Query().Where("created_at", query.OpGt, query.F{Path: "fixed_at", Offset: 0}),
			[]int{3},
		},
		{"field ref with offset", s.Query().
			Where("fixed_at", query.OpGt, query.F{Path: "created_at", Offset: -100 * 24 * time.Hour}), []int{3}},
		{
			"field ref numeric cross-type",
			s.Query().Where("score", query.OpLt, query.F{Path: "repo.owner.id", Offset: 0}),
			[]int{1, 2, 3},
		},
		{"anded conditions", s.Query().
			Where("state", query.OpEq, "open").
			Where("created_at", query.OpLt, ts("2026-01-15T00:00:00Z")), []int{1}},
		{"no match", s.Query().Where("state", query.OpEq, "dismissed"), []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.q.All()
			require.NoError(t, err)
			assert.Equal(t, tt.want, numbers(got))
		})
	}
}

func TestQuery_Where_errors(t *testing.T) {
	s := testSet()
	// A set whose source data only carried number, state and repo.full_name;
	// repo.owner exists in the schema but not in the loaded data.
	partial := query.NewSet([]entry{},
		map[string]bool{"number": true, "state": true, "repo": true, "repo.full_name": true})

	tests := []struct {
		name string
		q    *query.Query[entry]
		msg  string
	}{
		{"unknown field", s.Query().Where("stat", query.OpEq, "open"), `unknown field "stat"`},
		{"unknown nested field", s.Query().Where("repo.ownerz.id", query.OpEq, 1), `unknown field "ownerz"`},
		{
			"missing nested field",
			partial.Query().Where("repo.owner.id", query.OpEq, 1),
			`key "repo.owner.id" not present in the loaded data`,
		},
		{"path through non-struct", s.Query().Where("state.nope", query.OpEq, "x"), "is not a struct"},
		{"lt on string field", s.Query().Where("state", query.OpLt, "open"), "needs a numeric or time field"},
		{"value type mismatch", s.Query().Where("state", query.OpEq, 5), "incompatible with value type"},
		{
			"time field non-time value",
			s.Query().Where("created_at", query.OpLt, "2026-01-01"),
			"incompatible with value type",
		},
		{"in with non-slice", s.Query().Where("state", query.OpIn, "open"), "value must be a slice"},
		{
			"field ref unknown path",
			s.Query().Where("fixed_at", query.OpGt, query.F{Path: "bogus", Offset: 0}),
			`unknown field "bogus"`,
		},
		{
			"field ref incompatible classes",
			s.Query().Where("state", query.OpEq, query.F{Path: "score", Offset: 0}),
			"incompatible with referenced field",
		},
		{
			"field ref offset on non-time",
			s.Query().Where("score", query.OpLt, query.F{Path: "repo.owner.id", Offset: time.Hour}),
			"offset needs time fields",
		},
		{
			"field ref with in",
			s.Query().Where("state", query.OpIn, query.F{Path: "state", Offset: 0}),
			"does not accept a field reference",
		},
		{
			"field ref ordered op on string",
			s.Query().Where("state", query.OpLt, query.F{Path: "repo.full_name", Offset: 0}),
			"needs a numeric or time field",
		},
		{
			"field ref missing key",
			partial.Query().Where("number", query.OpEq, query.F{Path: "repo.owner.id", Offset: 0}),
			`key "repo.owner.id" not present in the loaded data`,
		},
		{"unknown op", s.Query().Where("state", query.Op("like"), "open"), `unknown op "like"`},
		{"first error wins", s.Query().
			Where("bogus", query.OpEq, "x").
			Where("also_bogus", query.OpEq, "y"), `unknown field "bogus"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.q.All()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.msg)

			_, err = tt.q.Count()
			require.Error(t, err)

			_, err = tt.q.CountBy("state")
			require.Error(t, err)
		})
	}
}

func TestQuery_immutable(t *testing.T) {
	s := testSet()

	base := s.Query().Where("created_at", query.OpLt, ts("2026-12-31T00:00:00Z"))
	open := base.Where("state", query.OpEq, "open")
	fixed := base.Where("state", query.OpEq, "fixed")

	openN, err := open.Count()
	require.NoError(t, err)
	fixedN, err := fixed.Count()
	require.NoError(t, err)
	baseN, err := base.Count()
	require.NoError(t, err)

	assert.Equal(t, 2, openN)
	assert.Equal(t, 1, fixedN)
	assert.Equal(t, 3, baseN, "branching must not mutate the base query")
}

func TestQuery_Count(t *testing.T) {
	s := testSet()

	n, err := s.Query().Where("state", query.OpEq, "open").Count()
	require.NoError(t, err)
	assert.Equal(t, 2, n)
}

func TestQuery_CountBy(t *testing.T) {
	s := testSet()

	t.Run("groups with total", func(t *testing.T) {
		gc, err := s.Query().CountBy("state")
		require.NoError(t, err)
		assert.Equal(t, map[string]int{"open": 2, "fixed": 1}, gc.Groups)
		assert.Equal(t, 3, gc.Total)
	})

	t.Run("zero-fill keys", func(t *testing.T) {
		gc, err := s.Query().Where("state", query.OpEq, "open").CountBy("state", "open", "fixed", "dismissed")
		require.NoError(t, err)
		assert.Equal(t, map[string]int{"open": 2, "fixed": 0, "dismissed": 0}, gc.Groups)
		assert.Equal(t, 2, gc.Total)
	})

	t.Run("nil pointer groups under empty key", func(t *testing.T) {
		gc, err := s.Query().CountBy("scope")
		require.NoError(t, err)
		assert.Equal(t, map[string]int{"": 2, "runtime": 1}, gc.Groups)
	})

	t.Run("non-string group field errors", func(t *testing.T) {
		_, err := s.Query().CountBy("score")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "group field must be a string")
	})
}

func TestQuery_Distinct(t *testing.T) {
	s := testSet()

	t.Run("unique sorted values", func(t *testing.T) {
		got, err := s.Query().Distinct("repo.full_name")
		require.NoError(t, err)
		assert.Equal(t, []string{"org/a", "org/b"}, got)
	})

	t.Run("respects conditions", func(t *testing.T) {
		got, err := s.Query().Where("state", query.OpEq, "open").Distinct("repo.full_name")
		require.NoError(t, err)
		assert.Equal(t, []string{"org/a"}, got)
	})

	t.Run("nil pointer becomes empty string", func(t *testing.T) {
		got, err := s.Query().Distinct("scope")
		require.NoError(t, err)
		assert.Equal(t, []string{"", "runtime"}, got)
	})

	t.Run("non-string field errors", func(t *testing.T) {
		_, err := s.Query().Distinct("score")
		require.Error(t, err)
	})
}

func TestQuery_CountByNested(t *testing.T) {
	s := testSet()

	got, err := s.Query().CountByNested("repo.full_name", "state", "open", "fixed")
	require.NoError(t, err)

	require.Len(t, got, 2)
	assert.Equal(t, map[string]int{"open": 2, "fixed": 0}, got["org/a"].Groups)
	assert.Equal(t, 2, got["org/a"].Total)
	assert.Equal(t, map[string]int{"open": 0, "fixed": 1}, got["org/b"].Groups)
	assert.Equal(t, 1, got["org/b"].Total)
}

func TestGroupCount_MarshalJSON(t *testing.T) {
	gc := query.GroupCount{Groups: map[string]int{"low": 1, "high": 2}, Total: 3}

	b, err := json.Marshal(gc)
	require.NoError(t, err)
	assert.JSONEq(t, `{"low":1,"high":2,"total":3}`, string(b))
}

func TestGroupCount_Decode(t *testing.T) {
	gc := query.GroupCount{Groups: map[string]int{"low": 1, "high": 2}, Total: 3}

	var out struct {
		Low   int `json:"low"`
		High  int `json:"high"`
		Total int `json:"total"`
	}
	require.NoError(t, gc.Decode(&out))
	assert.Equal(t, 1, out.Low)
	assert.Equal(t, 2, out.High)
	assert.Equal(t, 3, out.Total)
}

func TestQuery_sourceKeys(t *testing.T) {
	// Simulates data decoded from a source that only carried number and state
	// (e.g. json from an endpoint omitting other keys).
	var e1, e2 entry
	e1.Number, e1.State = 1, "open"
	e2.Number, e2.State = 2, "open"
	s := query.NewSet([]entry{e1, e2}, map[string]bool{"number": true, "state": true})

	t.Run("present key queries normally", func(t *testing.T) {
		got, err := s.Query().Where("state", query.OpEq, "open").Count()
		require.NoError(t, err)
		assert.Equal(t, 2, got)
	})

	t.Run("absent key errors in Where", func(t *testing.T) {
		_, err := s.Query().Where("repo.full_name", query.OpEq, "org/a").Count()
		require.Error(t, err)
		assert.Contains(t, err.Error(), `key "repo.full_name" not present in the loaded data`)
	})

	t.Run("absent key errors in Distinct", func(t *testing.T) {
		_, err := s.Query().Distinct("repo.full_name")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `key "repo.full_name" not present`)
	})

	t.Run("absent key errors in CountBy", func(t *testing.T) {
		_, err := s.Query().CountBy("repo.full_name")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `key "repo.full_name" not present`)
	})

	t.Run("absent key errors in CountByNested", func(t *testing.T) {
		_, err := s.Query().CountByNested("repo.full_name", "state")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `key "repo.full_name" not present`)

		_, err = s.Query().CountByNested("state", "repo.full_name")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `key "repo.full_name" not present`)
	})

	t.Run("nil keys mean in-memory data with every field present", func(t *testing.T) {
		got, err := testSet().Query().Distinct("repo.full_name")
		require.NoError(t, err)
		assert.Equal(t, []string{"org/a", "org/b"}, got)
	})
}
