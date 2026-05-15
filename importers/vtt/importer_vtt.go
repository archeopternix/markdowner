package vtt

import (
	"context"
	"fmt"
	"html"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	. "github.com/archeopternix/markdowner"
)

type VTTmporter struct{}

func New() *VTTmporter { return &VTTmporter{} }

func (*VTTmporter) Name() string { return "vtt" }

func (*VTTmporter) Accept(ctx context.Context, src ImportSource) bool {
	_ = ctx
	name := strings.ToLower(src.Name)
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".vtt":
		return true
	default:
		mt := strings.ToLower(strings.TrimSpace(src.MimeType))
		return mt == "text/vtt" || mt == "text/plain"
	}
}

// Import reads the markdown source, extracts frontmatter if present, and saves a Document to the store.
func (*VTTmporter) Import(ctx context.Context, store DocumentStore, src ImportSource) (*Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if store == nil {
		return nil, fmt.Errorf("store is nil")
	}
	if src.Reader == nil {
		return nil, fmt.Errorf("source reader is nil")
	}

	b, err := ioReadAllWithContext(ctx, src.Reader)
	if err != nil {
		return nil, err
	}

	bodyText := string(b)

	fm := parseFrontmatterMinimal(bodyText)

	now := time.Now().UTC()
	t := src.ModTime
	if t.IsZero() {
		t = now
	}
	stamp := t.Format("02.01.2006 15:04")

	// Defaults if missing or no frontmatter.
	titleDefault := strings.TrimSuffix(filepath.Base(src.Name), filepath.Ext(src.Name))
	if strings.TrimSpace(fm.Title) == "" {
		fm.Title = titleDefault
	}
	if strings.TrimSpace(fm.Date) == "" {
		fm.Date = stamp
	}
	if strings.TrimSpace(fm.ChangedDate) == "" {
		fm.ChangedDate = stamp
	}
	if strings.TrimSpace(fm.Version) == "" {
		fm.Version = "V1.0"
	}
	if strings.TrimSpace(fm.OriginalDocument) == "" {
		fm.OriginalDocument = src.Name
	}
	if strings.TrimSpace(fm.OriginalFormat) == "" {
		fm.OriginalFormat = "vtt"
	}

	slog.Info("Parsed VTT", "title", fm.Title, "date", fm.Date)

	bodyText, speakers := extractVTTBodyText(bodyText)
	if strings.TrimSpace(bodyText) == "" {
		// Create a minimal markdown document if none is present.
		// Keep it simple and deterministic.
		bodyText = "# " + fm.Title + "\n"
	}
	if len(speakers) > 0 {
		var b strings.Builder
		b.WriteString("## Speakers:\n")
		for _, speaker := range speakers {
			b.WriteString("* " + html.UnescapeString(speaker))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString("## Conversation:\n")
		b.WriteString(bodyText)
		bodyText = b.String()
	}

	doc := &Document{
		Frontmatter: fm,
		Markdown:    bodyText,
		Media:       nil,
	}

	if err := store.SaveOrUpdate(ctx, doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func ioReadAllWithContext(ctx context.Context, r io.Reader) ([]byte, error) {
	_ = ctx
	return io.ReadAll(r)
}

func extractVTTBodyText(bodyText string) (string, []string) {
	n := len(bodyText)
	if n == 0 {
		return "", nil
	}

	// Skip first line (VTT header).
	i := 0
	for i < n && bodyText[i] != '\n' {
		i++
	}
	if i < n {
		i++
	}

	// Skip blank lines until first content line.
	for i < n {
		start := i
		for i < n && bodyText[i] != '\n' {
			i++
		}
		end := i
		if i < n {
			i++
		}
		if !isBlankLine(bodyText[start:end]) {
			i = start
			break
		}
	}

	var b strings.Builder
	wrote := false
	var currentSpeaker string
	var currentText strings.Builder
	hasCurrent := false
	flushCurrent := func() {
		if !hasCurrent {
			return
		}
		if wrote {
			b.WriteString("\n")
		}
		if currentSpeaker != "" {
			b.WriteString(html.UnescapeString(currentSpeaker))
			b.WriteString(":\n")
		}
		b.WriteString(html.UnescapeString(currentText.String()))
		b.WriteString("\n")
		wrote = true
		currentSpeaker = ""
		currentText.Reset()
		hasCurrent = false
	}
	speakers := make([]string, 0, 8)
	seenSpeakers := make(map[string]struct{}, 8)

	for i < n {
		var ok bool
		i, ok = skipLine(bodyText, i) // cue id
		if !ok {
			break
		}
		i, ok = skipLine(bodyText, i) // time range
		if !ok {
			break
		}

		// Collect cue text until blank separator and normalize speaker tags.
		speaker := ""
		var cue strings.Builder
		cueHasText := false
		for i < n {
			start := i
			for i < n && bodyText[i] != '\n' {
				i++
			}
			end := i
			if i < n {
				i++
			}
			if isBlankLine(bodyText[start:end]) {
				break
			}
			if end > start && bodyText[end-1] == '\r' {
				end--
			}
			if end <= start {
				continue
			}

			line := bodyText[start:end]
			if strings.HasPrefix(line, "<v") {
				if gt := strings.IndexByte(line, '>'); gt >= 0 {
					if speaker == "" {
						speaker = strings.TrimSpace(line[2:gt])
						if speaker != "" {
							if _, ok := seenSpeakers[speaker]; !ok {
								seenSpeakers[speaker] = struct{}{}
								speakers = append(speakers, speaker)
							}
						}
					}
					line = line[gt+1:]
				}
			}

			if strings.Contains(line, "</v>") {
				line = strings.ReplaceAll(line, "</v>", "")
			}
			line = strings.TrimSpace(line)
			if line != "" {
				if cueHasText {
					cue.WriteByte(' ')
				}
				cue.WriteString(line)
				cueHasText = true
			}
		}

		if speaker != "" || cueHasText {
			cueText := cue.String()
			if hasCurrent && currentSpeaker == speaker {
				if cueText != "" {
					if currentText.Len() > 0 {
						currentText.WriteByte(' ')
					}
					currentText.WriteString(cueText)
				}
			} else {
				flushCurrent()
				currentSpeaker = speaker
				if cueText != "" {
					currentText.WriteString(cueText)
				}
				hasCurrent = true
			}
		}

		// Skip any additional blank lines between cues.
		for i < n {
			start := i
			for i < n && bodyText[i] != '\n' {
				i++
			}
			end := i
			if i < n {
				i++
			}
			if !isBlankLine(bodyText[start:end]) {
				i = start
				break
			}
		}
	}
	flushCurrent()

	return b.String(), speakers
}

func skipLine(s string, i int) (int, bool) {
	if i >= len(s) {
		return i, false
	}
	for i < len(s) && s[i] != '\n' {
		i++
	}
	if i < len(s) {
		i++
	}
	return i, true
}

func isBlankLine(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\r':
		default:
			return false
		}
	}
	return true
}

// parseFrontmatterMinimal parses a minimal YAML subset:
// - key: value pairs
// - keywords:\n  - item\n  - item
//
// Unknown keys are ignored.
func parseFrontmatterMinimal(yaml string) Frontmatter {
	lines := strings.Split(yaml, "\n")
	var fm Frontmatter
	inKeywords := false
	for _, ln := range lines {
		ln = strings.TrimRight(ln, "\r\t ")
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(ln), "keywords:") {
			inKeywords = true
			continue
		}
		if inKeywords {
			trim := strings.TrimSpace(ln)
			if strings.HasPrefix(trim, "-") {
				kw := strings.TrimSpace(strings.TrimPrefix(trim, "-"))
				fm.Keywords = append(fm.Keywords, unquoteYAML(kw))
				continue
			}
			inKeywords = false
		}

		k, v, ok := cutKeyValue(ln)
		if !ok {
			continue
		}
		k = strings.ToLower(k)
		v = unquoteYAML(strings.TrimSpace(v))
		switch k {
		case "author":
			fm.Author = v
		case "title":
			fm.Title = v
		case "subtitle":
			fm.Subtitle = v
		case "date":
			fm.Date = v
		case "changeddate", "changed_date", "changed-date":
			fm.ChangedDate = v
		case "originaldocument", "original_document", "original-document":
			fm.OriginalDocument = v
		case "originalformat", "original_format", "original-format":
			fm.OriginalFormat = v
		case "version":
			fm.Version = v
		case "language":
			fm.Language = v
		case "abstract":
			fm.Abstract = v
		}
	}
	return fm
}

func cutKeyValue(ln string) (key, val string, ok bool) {
	idx := strings.Index(ln, ":")
	if idx <= 0 {
		return "", "", false
	}
	key = strings.TrimSpace(ln[:idx])
	val = strings.TrimSpace(ln[idx+1:])
	if key == "" {
		return "", "", false
	}
	return key, val, true
}

func unquoteYAML(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
		s = strings.ReplaceAll(s, "\\\"", "\"")
		s = strings.ReplaceAll(s, "\\\\", "\\")
	}
	return s
}
