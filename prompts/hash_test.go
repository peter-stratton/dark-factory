package prompts

import "testing"

func TestHash_ReturnsHexDigest(t *testing.T) {
	got := Hash()
	if len(got) != 64 {
		t.Errorf("Hash() length = %d, want 64 hex chars", len(got))
	}
	for _, c := range got {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Hash() contains non-hex character %q: %s", c, got)
			break
		}
	}
}

func TestHash_Deterministic(t *testing.T) {
	first := Hash()
	second := Hash()
	if first != second {
		t.Errorf("Hash() not deterministic: %q vs %q", first, second)
	}
}
