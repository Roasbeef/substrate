package commands

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestDiffIdempotencyKey_Deterministic verifies that the same inputs always
// produce the same idempotency key.
func TestDiffIdempotencyKey_Deterministic(t *testing.T) {
	t.Parallel()

	patch := "diff --git a/foo.go b/foo.go\n+hello\n-world\n"
	key1 := diffIdempotencyKey(1, "User", patch)
	key2 := diffIdempotencyKey(1, "User", patch)

	require.Equal(t, key1, key2)
}

// TestDiffIdempotencyKey_DifferentPatchesDiffer verifies that different diff
// content produces different idempotency keys.
func TestDiffIdempotencyKey_DifferentPatchesDiffer(t *testing.T) {
	t.Parallel()

	patchA := "diff --git a/foo.go b/foo.go\n+hello\n"
	patchB := "diff --git a/foo.go b/foo.go\n+world\n"

	keyA := diffIdempotencyKey(1, "User", patchA)
	keyB := diffIdempotencyKey(1, "User", patchB)

	require.NotEqual(t, keyA, keyB)
}

// TestDiffIdempotencyKey_DifferentSendersDiffer verifies that the same diff
// from different senders produces different keys so agents don't collide.
func TestDiffIdempotencyKey_DifferentSendersDiffer(t *testing.T) {
	t.Parallel()

	patch := "diff --git a/foo.go b/foo.go\n+same\n"
	keyA := diffIdempotencyKey(1, "User", patch)
	keyB := diffIdempotencyKey(2, "User", patch)

	require.NotEqual(t, keyA, keyB)
}

// TestDiffIdempotencyKey_DifferentRecipientsDiffer verifies that the same
// diff to different recipients produces different keys.
func TestDiffIdempotencyKey_DifferentRecipientsDiffer(t *testing.T) {
	t.Parallel()

	patch := "diff --git a/foo.go b/foo.go\n+same\n"
	keyA := diffIdempotencyKey(1, "User", patch)
	keyB := diffIdempotencyKey(1, "Reviewer", patch)

	require.NotEqual(t, keyA, keyB)
}

// TestDiffIdempotencyKey_ColonInRecipient verifies that colons in the
// recipient name do not cause hash collisions due to delimiter ambiguity.
// This is prevented by length-prefixed encoding in the hash input.
func TestDiffIdempotencyKey_ColonInRecipient(t *testing.T) {
	t.Parallel()

	// Without length-prefixing, these would hash to the same input:
	// "diff:1:user:test:patch" vs "diff:1:user:test:patch"
	keyA := diffIdempotencyKey(1, "user:test", "patch1")
	keyB := diffIdempotencyKey(1, "user", ":test:patch1")

	require.NotEqual(t, keyA, keyB)
}

// TestDiffIdempotencyKey_HasDiffPrefix verifies that generated keys always
// start with the "diff:" prefix for identification.
func TestDiffIdempotencyKey_HasDiffPrefix(t *testing.T) {
	t.Parallel()

	key := diffIdempotencyKey(42, "User", "some patch")
	require.True(t, strings.HasPrefix(key, "diff:"))
}

// TestDiffIdempotencyKey_LengthConsistent verifies that the key length is
// consistent regardless of input size (prefix + 64 hex chars from SHA-256).
func TestDiffIdempotencyKey_LengthConsistent(t *testing.T) {
	t.Parallel()

	small := diffIdempotencyKey(1, "A", "x")
	large := diffIdempotencyKey(1, "A", strings.Repeat("x", 1_000_000))

	// "diff:" (5) + 64 hex chars = 69.
	require.Len(t, small, 69)
	require.Len(t, large, 69)
}

// TestDiffIdempotencyKey_Rapid uses property-based testing to verify that
// unique inputs always produce unique keys.
func TestDiffIdempotencyKey_Rapid(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		senderA := rapid.Int64Range(1, 1000).Draw(t, "senderA")
		senderB := rapid.Int64Range(1, 1000).Draw(t, "senderB")
		recipA := rapid.StringMatching(`[a-zA-Z]{1,20}`).Draw(t, "recipA")
		recipB := rapid.StringMatching(`[a-zA-Z]{1,20}`).Draw(t, "recipB")
		patchA := rapid.String().Draw(t, "patchA")
		patchB := rapid.String().Draw(t, "patchB")

		keyA := diffIdempotencyKey(senderA, recipA, patchA)
		keyB := diffIdempotencyKey(senderB, recipB, patchB)

		// Keys must differ if any input differs.
		if senderA != senderB || recipA != recipB || patchA != patchB {
			if keyA == keyB {
				t.Fatalf(
					"collision: sender=%d/%d recip=%s/%s "+
						"patch lengths=%d/%d",
					senderA, senderB, recipA, recipB,
					len(patchA), len(patchB),
				)
			}
		} else {
			// Identical inputs must produce identical keys.
			if keyA != keyB {
				t.Fatalf("determinism violated")
			}
		}
	})
}

// TestComputeDiffStats verifies that diff statistics are correctly extracted
// from unified diff patches.
func TestComputeDiffStats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		patch string
		want  diffStats
	}{
		{
			name:  "empty patch",
			patch: "",
			want:  diffStats{},
		},
		{
			name: "single file with additions and deletions",
			patch: "diff --git a/foo.go b/foo.go\n" +
				"--- a/foo.go\n" +
				"+++ b/foo.go\n" +
				"-old line\n" +
				"+new line\n" +
				"+another new\n",
			want: diffStats{
				Files: 1, Additions: 2, Deletions: 1,
			},
		},
		{
			name: "multiple files",
			patch: "diff --git a/foo.go b/foo.go\n" +
				"+added\n" +
				"diff --git a/bar.go b/bar.go\n" +
				"-removed\n" +
				"+replaced\n",
			want: diffStats{
				Files: 2, Additions: 2, Deletions: 1,
			},
		},
		{
			name: "only additions",
			patch: "diff --git a/new.go b/new.go\n" +
				"+line1\n+line2\n+line3\n",
			want: diffStats{
				Files: 1, Additions: 3, Deletions: 0,
			},
		},
		{
			name: "only deletions",
			patch: "diff --git a/old.go b/old.go\n" +
				"-line1\n-line2\n",
			want: diffStats{
				Files: 1, Additions: 0, Deletions: 2,
			},
		},
		{
			name: "diff header lines not counted",
			patch: "diff --git a/x.go b/x.go\n" +
				"--- a/x.go\n" +
				"+++ b/x.go\n" +
				"@@ -1,3 +1,3 @@\n",
			want: diffStats{
				Files: 1, Additions: 0, Deletions: 0,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := computeDiffStats(tc.patch)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestDiffStats_Summary verifies the human-readable summary format.
func TestDiffStats_Summary(t *testing.T) {
	t.Parallel()

	s := diffStats{Files: 3, Additions: 42, Deletions: 7}
	require.Equal(t, "3 files, +42/-7 lines", s.summary())
}

// TestDiffStats_Summary_Zero verifies that zero-value stats produce a
// sensible summary.
func TestDiffStats_Summary_Zero(t *testing.T) {
	t.Parallel()

	s := diffStats{}
	require.Equal(t, "0 files, +0/-0 lines", s.summary())
}
