package envexpand

import (
	"errors"
	"testing"
)

func TestExpandReplacesSetVars(t *testing.T) {
	t.Setenv("SAPPER_TEST_ONE", "1")
	t.Setenv("SAPPER_TEST_TWO", "two")
	got, err := Expand("a=${SAPPER_TEST_ONE} b=${SAPPER_TEST_TWO} c=${SAPPER_TEST_ONE}")
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if want := "a=1 b=two c=1"; got != want {
		t.Errorf("Expand() = %q, want %q", got, want)
	}
}

func TestExpandEmptyValueIsNotMissing(t *testing.T) {
	t.Setenv("SAPPER_TEST_EMPTY", "")
	got, err := Expand("x=${SAPPER_TEST_EMPTY}")
	if err != nil {
		t.Fatalf("Expand() error = %v, want nil (set-but-empty is not missing)", err)
	}
	if got != "x=" {
		t.Errorf("Expand() = %q, want %q", got, "x=")
	}
}

func TestExpandUnsetVarsReportedAllAtOnceAndDeduped(t *testing.T) {
	_, err := Expand("${SAPPER_TEST_ABSENT_A} ${SAPPER_TEST_ABSENT_B} ${SAPPER_TEST_ABSENT_A}")
	var missing *MissingVarsError
	if !errors.As(err, &missing) {
		t.Fatalf("Expand() error = %v, want *MissingVarsError", err)
	}
	if len(missing.Names) != 2 {
		t.Fatalf("MissingVarsError.Names = %v, want two distinct names", missing.Names)
	}
}

func TestExpandLeavesNonBraceAndInvalidFormsLiteral(t *testing.T) {
	const in = "$NOT_EXPANDED ${9invalid} ${} literal"
	got, err := Expand(in)
	if err != nil {
		t.Fatalf("Expand() error = %v, want nil", err)
	}
	if got != in {
		t.Errorf("Expand() = %q, want it unchanged %q", got, in)
	}
}
