package agent

import "testing"

func TestExtractCloneSHA(t *testing.T) {
	validSHA := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"

	tests := []struct {
		name   string
		stdout string
		want   string
	}{
		{
			name:   "valid SHA on its own line",
			stdout: "CLONE_SHA=" + validSHA + "\n",
			want:   validSHA,
		},
		{
			name:   "valid SHA preceded by clone output",
			stdout: "Cloning into '/workspace'...\nCLONE_SHA=" + validSHA + "\n",
			want:   validSHA,
		},
		{
			name:   "valid SHA followed by agent JSON output",
			stdout: "CLONE_SHA=" + validSHA + "\n{\"type\":\"result\",\"session_id\":\"abc\"}\n",
			want:   validSHA,
		},
		{
			name:   "missing sentinel line",
			stdout: "Cloning into '/workspace'...\nsome other output\n",
			want:   "",
		},
		{
			name:   "SHA too short (39 chars)",
			stdout: "CLONE_SHA=a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1\n",
			want:   "",
		},
		{
			name:   "SHA too long (41 chars)",
			stdout: "CLONE_SHA=a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c\n",
			want:   "",
		},
		{
			name:   "SHA with uppercase letters should not match",
			stdout: "CLONE_SHA=A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4E5F6A1B2\n",
			want:   "",
		},
		{
			name:   "empty stdout",
			stdout: "",
			want:   "",
		},
		{
			name:   "SHA embedded in larger word does not match",
			stdout: "PREFIX_CLONE_SHA=" + validSHA + "\n",
			want:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractCloneSHA(tc.stdout)
			if got != tc.want {
				t.Errorf("extractCloneSHA() = %q, want %q", got, tc.want)
			}
		})
	}
}
