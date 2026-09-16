package db

import "testing"

func TestDialectPlaceholders(t *testing.T) {
	cases := []struct {
		driver string
		pos    int
		want   string
	}{
		{"POSTGRES", 1, "$1"},
		{"POSTGRES", 2, "$2"},
		{"ORACLE", 1, ":1"},
		{"MSSQL", 1, "@p1"},
	}
	for _, c := range cases {
		d, ok := dialectFor(c.driver)
		if !ok {
			t.Fatalf("dialectFor(%q): not found", c.driver)
		}
		if got := d.Placeholder(c.pos); got != c.want {
			t.Errorf("%s.Placeholder(%d) = %q, want %q", c.driver, c.pos, got, c.want)
		}
	}
}

func TestDialectForUnknown(t *testing.T) {
	if _, ok := dialectFor("DB2"); ok {
		t.Error("dialectFor(\"DB2\") should not be found")
	}
}
