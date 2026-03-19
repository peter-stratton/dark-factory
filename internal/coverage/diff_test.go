package coverage

import (
	"reflect"
	"testing"
)

func TestParseUnifiedDiff_Basic(t *testing.T) {
	diff := `diff --git a/internal/foo/bar.go b/internal/foo/bar.go
--- a/internal/foo/bar.go
+++ b/internal/foo/bar.go
@@ -10,0 +11,5 @@ func existing() {
+func newFunc() {
+	a := 1
+	b := 2
+	return a + b
+}
@@ -30,0 +36,3 @@ func another() {
+	x := 10
+	y := 20
+	return x + y
`

	got := ParseUnifiedDiff(diff)

	want := map[string][]LineRange{
		"internal/foo/bar.go": {
			{Start: 11, End: 15},
			{Start: 36, End: 38},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseUnifiedDiff() = %v, want %v", got, want)
	}
}

func TestParseUnifiedDiff_MultipleFiles(t *testing.T) {
	diff := `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -5,0 +6,2 @@
+line1
+line2
diff --git a/b.go b/b.go
--- a/b.go
+++ b/b.go
@@ -1,0 +2,1 @@
+newline
`

	got := ParseUnifiedDiff(diff)

	want := map[string][]LineRange{
		"a.go": {{Start: 6, End: 7}},
		"b.go": {{Start: 2, End: 2}},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseUnifiedDiff() = %v, want %v", got, want)
	}
}

func TestParseUnifiedDiff_FiltersTestFiles(t *testing.T) {
	diff := `diff --git a/foo.go b/foo.go
--- a/foo.go
+++ b/foo.go
@@ -1,0 +2,1 @@
+added
diff --git a/foo_test.go b/foo_test.go
--- a/foo_test.go
+++ b/foo_test.go
@@ -1,0 +2,5 @@
+test lines
+test lines
+test lines
+test lines
+test lines
`

	got := ParseUnifiedDiff(diff)

	want := map[string][]LineRange{
		"foo.go": {{Start: 2, End: 2}},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseUnifiedDiff() = %v, want %v", got, want)
	}
}

func TestParseUnifiedDiff_DeletionOnly(t *testing.T) {
	// A hunk with +N,0 means no lines were added.
	diff := `diff --git a/foo.go b/foo.go
--- a/foo.go
+++ b/foo.go
@@ -10,3 +10,0 @@
-deleted1
-deleted2
-deleted3
`

	got := ParseUnifiedDiff(diff)
	if got != nil {
		t.Errorf("ParseUnifiedDiff() = %v, want nil for deletion-only diff", got)
	}
}

func TestParseUnifiedDiff_Empty(t *testing.T) {
	got := ParseUnifiedDiff("")
	if got != nil {
		t.Errorf("ParseUnifiedDiff(\"\") = %v, want nil", got)
	}
}

func TestParseUnifiedDiff_SingleLineAdd(t *testing.T) {
	// When count is omitted, it defaults to 1.
	diff := `diff --git a/foo.go b/foo.go
--- a/foo.go
+++ b/foo.go
@@ -5,0 +6 @@
+new line
`

	got := ParseUnifiedDiff(diff)

	want := map[string][]LineRange{
		"foo.go": {{Start: 6, End: 6}},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseUnifiedDiff() = %v, want %v", got, want)
	}
}
