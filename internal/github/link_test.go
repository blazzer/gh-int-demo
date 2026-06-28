package github

import "testing"

func TestParseLinkNext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   string
		ok     bool
	}{
		{
			name:   "next link",
			header: `<https://api.github.com/user/repos?page=2>; rel="next", <https://api.github.com/user/repos?page=5>; rel="last"`,
			want:   "https://api.github.com/user/repos?page=2",
			ok:     true,
		},
		{
			name:   "no next",
			header: `<https://api.github.com/user/repos?page=1>; rel="prev"`,
			want:   "",
			ok:     false,
		},
		{
			name:   "empty",
			header: "",
			want:   "",
			ok:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseLinkNext(tt.header)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("parseLinkNext(%q) = (%q, %v), want (%q, %v)", tt.header, got, ok, tt.want, tt.ok)
			}
		})
	}
}
