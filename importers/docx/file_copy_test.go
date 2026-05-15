package docx

import "testing"

func TestRewritePandocFigureImageHTML(t *testing.T) {
	in := `<figure>
<img src="C:\Users\ANDREA~1\AppData\Local\Temp\media1776054263/media/image1.png" style="width:4.94148in;height:3.05521in" alt="Image: Responsibility of Roles" />
<figcaption aria-hidden="true"><p>Image: Responsibility of Roles</p></figcaption>
</figure>`

	got := rewritePandocFigureImageHTML(in, "media/")
	want := `![Image: Responsibility of Roles](media/image1.png)`

	if got != want {
		t.Fatalf("unexpected rewrite.\nwant: %s\ngot:  %s", want, got)
	}
}

func TestRewritePandocFigureImageHTML_NoMediaPath(t *testing.T) {
	in := `<figure><img src="/tmp/not-media-dir/image1.png" alt="Preview" /></figure>`
	got := rewritePandocFigureImageHTML(in, "media/")
	want := `![Preview](/tmp/not-media-dir/image1.png)`

	if got != want {
		t.Fatalf("unexpected rewrite.\nwant: %s\ngot:  %s", want, got)
	}
}
