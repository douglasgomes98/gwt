package cli

import (
	"reflect"
	"testing"
)

func TestFlagsFirstKeepsAliasStyle(t *testing.T) {
	got := flagsFirst([]string{"AG-1", "main", "-e", "--all"})
	want := []string{"-e", "--all", "AG-1", "main"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
