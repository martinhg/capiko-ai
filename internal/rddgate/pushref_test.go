package rddgate

import (
	"strings"
	"testing"
)

func TestParsePushRefs(t *testing.T) {
	tests := []struct {
		name    string
		stdin   string
		want    []PushRef
		wantErr bool
	}{
		{
			name:  "normal single ref",
			stdin: "refs/heads/main abc123 refs/heads/main def456\n",
			want: []PushRef{
				{LocalRef: "refs/heads/main", LocalSHA: "abc123", RemoteRef: "refs/heads/main", RemoteSHA: "def456"},
			},
		},
		{
			name:  "delete-ref (local sha all zeros)",
			stdin: "(delete) 0000000000000000000000000000000000000000 refs/heads/feature abc123\n",
			want: []PushRef{
				{LocalRef: "(delete)", LocalSHA: "0000000000000000000000000000000000000000", RemoteRef: "refs/heads/feature", RemoteSHA: "abc123"},
			},
		},
		{
			name:  "force-push (unrelated shas, still a normal 4-field line)",
			stdin: "refs/heads/main aaa111 refs/heads/main bbb222\n",
			want: []PushRef{
				{LocalRef: "refs/heads/main", LocalSHA: "aaa111", RemoteRef: "refs/heads/main", RemoteSHA: "bbb222"},
			},
		},
		{
			name: "multi-ref",
			stdin: "refs/heads/main aaa111 refs/heads/main bbb222\n" +
				"refs/heads/dev ccc333 refs/heads/dev ddd444\n",
			want: []PushRef{
				{LocalRef: "refs/heads/main", LocalSHA: "aaa111", RemoteRef: "refs/heads/main", RemoteSHA: "bbb222"},
				{LocalRef: "refs/heads/dev", LocalSHA: "ccc333", RemoteRef: "refs/heads/dev", RemoteSHA: "ddd444"},
			},
		},
		{
			name:  "new-branch (remote sha all zeros)",
			stdin: "refs/heads/feature abc123 refs/heads/feature 0000000000000000000000000000000000000000\n",
			want: []PushRef{
				{LocalRef: "refs/heads/feature", LocalSHA: "abc123", RemoteRef: "refs/heads/feature", RemoteSHA: "0000000000000000000000000000000000000000"},
			},
		},
		{
			name:  "empty stdin",
			stdin: "",
			want:  []PushRef{},
		},
		{
			name:    "malformed line (too few fields)",
			stdin:   "refs/heads/main abc123 refs/heads/main\n",
			wantErr: true,
		},
		{
			name:    "malformed line (too many fields)",
			stdin:   "refs/heads/main abc123 refs/heads/main def456 extra\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePushRefs(strings.NewReader(tt.stdin))

			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParsePushRefs(%q) error = nil, want error", tt.stdin)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePushRefs(%q) unexpected error: %v", tt.stdin, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ParsePushRefs(%q) = %d refs, want %d: got=%+v", tt.stdin, len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("ParsePushRefs(%q)[%d] = %+v, want %+v", tt.stdin, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestIsDeleteRef(t *testing.T) {
	tests := []struct {
		name string
		ref  PushRef
		want bool
	}{
		{
			name: "local sha all zeros is a delete",
			ref:  PushRef{LocalRef: "(delete)", LocalSHA: "0000000000000000000000000000000000000000", RemoteRef: "refs/heads/feature", RemoteSHA: "abc123"},
			want: true,
		},
		{
			name: "normal local sha is not a delete",
			ref:  PushRef{LocalRef: "refs/heads/main", LocalSHA: "abc123", RemoteRef: "refs/heads/main", RemoteSHA: "def456"},
			want: false,
		},
		{
			name: "remote sha all zeros (new branch) is not a delete",
			ref:  PushRef{LocalRef: "refs/heads/feature", LocalSHA: "abc123", RemoteRef: "refs/heads/feature", RemoteSHA: "0000000000000000000000000000000000000000"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDeleteRef(tt.ref); got != tt.want {
				t.Fatalf("IsDeleteRef(%+v) = %v, want %v", tt.ref, got, tt.want)
			}
		})
	}
}
