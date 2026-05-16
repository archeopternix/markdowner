package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/archeopternix/markdowner/store"
)

func TestPublicTextImportStability(t *testing.T) {
	t.Log("Purpose: ensure plain-text import stays stable via the public API, including default frontmatter mapping and body persistence.")

	ctx := context.Background()
	storeRoot := t.TempDir()
	store := store.NewDocumentStore(store.NewLocalStoreFS(storeRoot))

	doc, err := store.ParseFromPath(ctx, fixturePath(t, "playbook.txt"))
	if err != nil {
		t.Fatalf("ParseFromPath(playbook.txt) failed: %v", err)
	}

	if strings.TrimSpace(doc.ID) == "" {
		t.Fatalf("expected non-empty doc ID")
	}
	if got, want := doc.Frontmatter.Title, "playbook"; got != want {
		t.Fatalf("unexpected title: got %q want %q", got, want)
	}
	if got, want := doc.Frontmatter.OriginalDocument, "playbook.txt"; got != want {
		t.Fatalf("unexpected original document: got %q want %q", got, want)
	}
	if got, want := doc.Frontmatter.OriginalFormat, "markdown"; got != want {
		t.Fatalf("unexpected original format: got %q want %q", got, want)
	}
	if !strings.Contains(doc.Markdown, "Welcome to today's Deep Dive") {
		t.Fatalf("expected imported markdown to contain known phrase from playbook transcript")
	}
	if len(doc.Media) != 0 {
		t.Fatalf("expected no media for text import, got %d entries", len(doc.Media))
	}
	if got, want := doc.Path, filepath.Join(storeRoot, doc.ID); got != want {
		t.Fatalf("unexpected document path: got %q want %q", got, want)
	}
}

func TestPublicVTTConferenceImportStability(t *testing.T) {
	t.Log("Purpose: ensure VTT conference recordings are transformed into structured markdown with speaker and conversation sections.")

	ctx := context.Background()
	store := store.NewDocumentStore(store.NewLocalStoreFS(t.TempDir()))

	doc, err := store.ParseFromPath(ctx, fixturePath(t, "sales.vtt"))
	if err != nil {
		t.Fatalf("ParseFromPath(sales.vtt) failed: %v", err)
	}

	if got, want := doc.Frontmatter.Title, "sales"; got != want {
		t.Fatalf("unexpected title: got %q want %q", got, want)
	}
	if got, want := doc.Frontmatter.OriginalFormat, "vtt"; got != want {
		t.Fatalf("unexpected original format: got %q want %q", got, want)
	}
	if !strings.Contains(doc.Markdown, "## Speakers:") {
		t.Fatalf("expected speakers section in markdown output")
	}
	if !strings.Contains(doc.Markdown, "## Conversation:") {
		t.Fatalf("expected conversation section in markdown output")
	}
	if !strings.Contains(doc.Markdown, "Maas, Matthias") {
		t.Fatalf("expected known speaker to be present in markdown output")
	}
	if !strings.Contains(doc.Markdown, "Hot dog.") {
		t.Fatalf("expected known conversation content to be present in markdown output")
	}
}

func TestPublicMarkdownImporterStability(t *testing.T) {
	t.Log("Purpose: ensure markdown importer keeps frontmatter values and strips frontmatter from the resulting markdown body.")

	ctx := context.Background()
	store := store.NewDocumentStore(store.NewLocalStoreFS(t.TempDir()))

	doc, err := store.ParseFromPath(ctx, fixturePath(t, "sample.md"))
	if err != nil {
		t.Fatalf("ParseFromPath(sample.md) failed: %v", err)
	}

	if got, want := doc.Frontmatter.Title, "Meeting Minutes 08.05.2026 07:00"; got != want {
		t.Fatalf("unexpected title: got %q want %q", got, want)
	}
	if got, want := doc.Frontmatter.Date, "08.05.2026 07:00"; got != want {
		t.Fatalf("unexpected date: got %q want %q", got, want)
	}
	if got, want := doc.Frontmatter.OriginalDocument, "sample.md"; got != want {
		t.Fatalf("unexpected original document: got %q want %q", got, want)
	}
	if got, want := doc.Frontmatter.OriginalFormat, "markdown"; got != want {
		t.Fatalf("unexpected original format: got %q want %q", got, want)
	}
	if strings.Contains(doc.Markdown, "---") {
		t.Fatalf("expected markdown body without frontmatter delimiters")
	}
	if !strings.Contains(doc.Markdown, "## Participants") {
		t.Fatalf("expected participants section in imported markdown body")
	}
}

func TestPublicDOCXImporterStability(t *testing.T) {
	t.Log("Purpose: ensure DOCX import remains available via the public API, preserving core content and extracted media files.")

	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc not found in PATH; skipping DOCX importer stability test")
	}

	ctx := context.Background()
	storeRoot := t.TempDir()
	store := store.NewDocumentStore(store.NewLocalStoreFS(storeRoot))

	doc, err := store.ParseFromPath(ctx, fixturePath(t, "shop.docx"))
	if err != nil {
		t.Fatalf("ParseFromPath(shop.docx) failed: %v", err)
	}

	if got, want := doc.Frontmatter.OriginalDocument, "shop.docx"; got != want {
		t.Fatalf("unexpected original document: got %q want %q", got, want)
	}
	if got, want := doc.Frontmatter.OriginalFormat, "docx"; got != want {
		t.Fatalf("unexpected original format: got %q want %q", got, want)
	}
	if !strings.Contains(doc.Markdown, "Management Summary") {
		t.Fatalf("expected known DOCX section heading in markdown output")
	}
	if !strings.Contains(doc.Markdown, "Applications → Merchandise Shop") {
		t.Fatalf("expected known business content in markdown output")
	}
	if len(doc.Media) == 0 {
		t.Fatalf("expected extracted media from DOCX, got none")
	}
	if !containsString(doc.Media, "image1.png") {
		t.Fatalf("expected media list to contain image1.png, got: %v", doc.Media)
	}

	mediaPath := filepath.Join(storeRoot, doc.ID, "media", "image1.png")
	if _, err := os.Stat(mediaPath); err != nil {
		t.Fatalf("expected extracted media file to be readable at %s: %v", mediaPath, err)
	}
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("unable to determine test file path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "testdata", name)
}

func containsString(items []string, want string) bool {
	for _, it := range items {
		if it == want {
			return true
		}
	}
	return false
}
