package migrate

import "testing"

func TestVersionOf(t *testing.T) {
	t.Parallel()
	v, err := versionOf("0001_init.sql")
	if err != nil || v != 1 {
		t.Fatalf("%d %v", v, err)
	}
	if _, err := versionOf("init.sql"); err == nil {
		t.Fatal("expected error")
	}
}
