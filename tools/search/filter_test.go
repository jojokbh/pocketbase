package search_test

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jojokbh/pocketbase/tools/search"
	"github.com/pocketbase/dbx"
)

func TestFilterDataBuildExpr(t *testing.T) {
	resolver := search.NewSimpleFieldResolver("test1", "test2", "test3", `^test4.\w+$`)

	scenarios := []struct {
		name          string
		filterData    search.FilterData
		expectError   bool
		expectPattern string
	}{
		{
			"empty",
			"",
			true,
			"",
		},
		{
			"invalid format",
			"(test1 > 1",
			true,
			"",
		},
		{
			"invalid operator",
			"test1 + 123",
			true,
			"",
		},
		{
			"unknown field",
			"test1 = 'example' && unknown > 1",
			true,
			"",
		},
		{
			"simple expression",
			"test1 > 1",
			false,
			"[[test1]] > {:TEST}",
		},
		{
			"empty string vs null",
			"'' = null && null != ''",
			false,
			"('' = '' AND '' != '')",
		},
		{
			"like with 2 columns",
			"test1 ~ test2",
			false,
			"[[test1]] LIKE ('%' || [[test2]] || '%') ESCAPE '\\'",
		},
		{
			"like with right column operand",
			"'lorem' ~ test1",
			false,
			"{:TEST} LIKE ('%' || [[test1]] || '%') ESCAPE '\\'",
		},
		{
			"like with left column operand and text as right operand",
			"test1 ~ 'lorem'",
			false,
			"[[test1]] LIKE {:TEST} ESCAPE '\\'",
		},
		{
			"not like with 2 columns",
			"test1 !~ test2",
			false,
			"[[test1]] NOT LIKE ('%' || [[test2]] || '%') ESCAPE '\\'",
		},
		{
			"not like with right column operand",
			"'lorem' !~ test1",
			false,
			"{:TEST} NOT LIKE ('%' || [[test1]] || '%') ESCAPE '\\'",
		},
		{
			"like with left column operand and text as right operand",
			"test1 !~ 'lorem'",
			false,
			"[[test1]] NOT LIKE {:TEST} ESCAPE '\\'",
		},
		{
			"macros",
			`
				test4.1 > @now &&
				test4.2 > @second &&
				test4.3 > @minute &&
				test4.4 > @hour &&
				test4.5 > @day &&
				test4.6 > @year &&
				test4.7 > @month &&
				test4.9 > @weekday &&
				test4.9 > @todayStart &&
				test4.10 > @todayEnd &&
				test4.11 > @monthStart &&
				test4.12 > @monthEnd &&
				test4.13 > @yearStart &&
				test4.14 > @yearEnd
			`,
			false,
			"([[test4.1]] > {:TEST} AND [[test4.2]] > {:TEST} AND [[test4.3]] > {:TEST} AND [[test4.4]] > {:TEST} AND [[test4.5]] > {:TEST} AND [[test4.6]] > {:TEST} AND [[test4.7]] > {:TEST} AND [[test4.9]] > {:TEST} AND [[test4.9]] > {:TEST} AND [[test4.10]] > {:TEST} AND [[test4.11]] > {:TEST} AND [[test4.12]] > {:TEST} AND [[test4.13]] > {:TEST} AND [[test4.14]] > {:TEST})",
		},
		{
			"complex expression",
			"((test1 > 1) || (test2 != 2)) && test3 ~ '%%example' && test4.sub = null",
			false,
			"(([[test1]] > {:TEST} OR [[test2]] != {:TEST}) AND [[test3]] LIKE {:TEST} ESCAPE '\\' AND ([[test4.sub]] = '' OR [[test4.sub]] IS NULL))",
		},
		{
			"combination of special literals (null, true, false)",
			"test1=true && test2 != false && null = test3 || null != test4.sub",
			false,
			"([[test1]] = 1 AND [[test2]] != 0 AND ('' = [[test3]] OR [[test3]] IS NULL) OR ('' != [[test4.sub]] AND [[test4.sub]] IS NOT NULL))",
		},
		{
			"all operators",
			"(test1 = test2 || test2 != test3) && (test2 ~ 'example' || test2 !~ '%%abc') && 'switch1%%' ~ test1 && 'switch2' !~ test2 && test3 > 1 && test3 >= 0 && test3 <= 4 && 2 < 5",
			false,
			"((COALESCE([[test1]], '') = COALESCE([[test2]], '') OR COALESCE([[test2]], '') != COALESCE([[test3]], '')) AND ([[test2]] LIKE {:TEST} ESCAPE '\\' OR [[test2]] NOT LIKE {:TEST} ESCAPE '\\') AND {:TEST} LIKE ('%' || [[test1]] || '%') ESCAPE '\\' AND {:TEST} NOT LIKE ('%' || [[test2]] || '%') ESCAPE '\\' AND [[test3]] > {:TEST} AND [[test3]] >= {:TEST} AND [[test3]] <= {:TEST} AND {:TEST} < {:TEST})",
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			expr, err := s.filterData.BuildExpr(resolver)

			hasErr := err != nil
			if hasErr != s.expectError {
				t.Fatalf("[%s] Expected hasErr %v, got %v (%v)", s.name, s.expectError, hasErr, err)
			}

			if hasErr {
				return
			}

			dummyDB := &dbx.DB{}

			rawSql := expr.Build(dummyDB, dbx.Params{})

			// replace TEST placeholder with .+ regex pattern
			expectPattern := strings.ReplaceAll(
				"^"+regexp.QuoteMeta(s.expectPattern)+"$",
				"TEST",
				`\w+`,
			)

			pattern := regexp.MustCompile(expectPattern)
			if !pattern.MatchString(rawSql) {
				t.Fatalf("[%s] Pattern %v don't match with expression: \n%v", s.name, expectPattern, rawSql)
			}
		})
	}
}

func TestFilterDataBuildExprWithParams(t *testing.T) {
	// create a dummy db
	sqlDB, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	db := dbx.NewFromDB(sqlDB, "sqlite")

	calledQueries := []string{}
	db.QueryLogFunc = func(ctx context.Context, t time.Duration, sql string, rows *sql.Rows, err error) {
		calledQueries = append(calledQueries, sql)
	}
	db.ExecLogFunc = func(ctx context.Context, t time.Duration, sql string, result sql.Result, err error) {
		calledQueries = append(calledQueries, sql)
	}

	date, err := time.Parse("2006-01-02", "2023-01-01")
	if err != nil {
		t.Fatal(err)
	}

	resolver := search.NewSimpleFieldResolver(`^test\w+$`)

	filter := search.FilterData(`
		test1 = {:test1} ||
		test2 = {:test2} ||
		test3a = {:test3} ||
		test3b = {:test3} ||
		test4 = {:test4} ||
		test5 = {:test5} ||
		test6 = {:test6} ||
		test7 = {:test7} ||
		test8 = {:test8} ||
		test9 = {:test9} ||
		test10 = {:test10} ||
		test11 = {:test11} ||
		test12 = {:test12}
	`)

	replacements := []dbx.Params{
		{"test1": true},
		{"test2": false},
		{"test3": 123.456},
		{"test4": nil},
		{"test5": "", "test6": "simple", "test7": `'single_quotes'`, "test8": `"double_quotes"`, "test9": `escape\"quote`},
		{"test10": date},
		{"test11": []string{"a", "b", `"quote`}},
		{"test12": map[string]any{"a": 123, "b": `quote"`}},
	}

	expr, err := filter.BuildExpr(resolver, replacements...)
	if err != nil {
		t.Fatal(err)
	}

	db.Select().Where(expr).Build().Execute()

	if len(calledQueries) != 1 {
		t.Fatalf("Expected 1 query, got %d", len(calledQueries))
	}

	expectedQuery := `SELECT * WHERE ([[test1]] = 1 OR [[test2]] = 0 OR [[test3a]] = 123.456 OR [[test3b]] = 123.456 OR ([[test4]] = '' OR [[test4]] IS NULL) OR [[test5]] = '""' OR [[test6]] = 'simple' OR [[test7]] = '''single_quotes''' OR [[test8]] = '"double_quotes"' OR [[test9]] = 'escape\\"quote' OR [[test10]] = '2023-01-01 00:00:00 +0000 UTC' OR [[test11]] = '["a","b","\\"quote"]' OR [[test12]] = '{"a":123,"b":"quote\\""}')`
	if expectedQuery != calledQueries[0] {
		t.Fatalf("Expected query \n%s, \ngot \n%s", expectedQuery, calledQueries[0])
	}
}
