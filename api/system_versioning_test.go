package api

import "testing"

func TestIsStaleBaseVersion(t *testing.T) {
	tests := []struct {
		name             string
		baseVersionID    string
		baseConfigHash   string
		activeVersionID  string
		activeConfigHash string
		want             bool
	}{
		{
			name:             "same hash different ids is not stale",
			baseVersionID:    "cv-1",
			baseConfigHash:   "abc",
			activeVersionID:  "cv-2",
			activeConfigHash: "abc",
			want:             false,
		},
		{
			name:             "different hash is stale",
			baseVersionID:    "cv-1",
			baseConfigHash:   "abc",
			activeVersionID:  "cv-2",
			activeConfigHash: "def",
			want:             true,
		},
		{
			name:            "fallback to id comparison when hashes missing",
			baseVersionID:   "cv-1",
			activeVersionID: "cv-2",
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isStaleBaseVersion(tt.baseVersionID, tt.baseConfigHash, tt.activeVersionID, tt.activeConfigHash)
			if got != tt.want {
				t.Fatalf("isStaleBaseVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
