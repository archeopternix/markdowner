package docx

import "testing"

func TestRewriteMarkdownMediaLinks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        string
		newPrefix string
		want      string
	}{
		{
			name:      "rewrite figure img with explicit alt",
			in:        `<p>before</p><figure><img src="foo/bar/photo.png" alt="My Photo" /></figure><p>after</p>`,
			newPrefix: "media/",
			want:      `<p>before</p>![My Photo](media/photo.png)<p>after</p>`,
		},
		{
			name:      "rewrite standalone img with fallback alt",
			in:        `<img src="images/diagram-final.jpeg">`,
			newPrefix: "assets",
			want:      `![diagram-final](assets/diagram-final.jpeg)`,
		},
		{
			name:      "keep random internet link unchanged",
			in:        `<img src="https://random.example.org/a/b/cat.png" alt="remote-cat" />`,
			newPrefix: "media",
			want:      `<img src="https://random.example.org/a/b/cat.png" alt="remote-cat" />`,
		},
		{
			name:      "keep non image link unchanged",
			in:        `<img src="docs/README.pdf" alt="manual" />`,
			newPrefix: "media",
			want:      `<img src="docs/README.pdf" alt="manual" />`,
		},
		{
			name:      "mixed content rewrites only local image tags",
			in:        `<figure><img src="/tmp/pic.webp" /></figure> x <img src="//cdn.example.net/r.png" alt="cdn"/> y <img src="a/b/logo.svg" alt="Logo"/>`,
			newPrefix: "m",
			want:      `![pic](m/pic.webp) x <img src="//cdn.example.net/r.png" alt="cdn"/> y ![Logo](m/logo.svg)`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := rewriteMarkdownMediaLinks(tc.in, tc.newPrefix)
			if got != tc.want {
				t.Fatalf("unexpected output\nin:   %q\nwant: %q\ngot:  %q", tc.in, tc.want, got)
			}
		})
	}
}
