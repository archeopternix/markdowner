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

	md "github.com/archeopternix/markdowner"
	"github.com/archeopternix/markdowner/internal/format"
	fsutils "github.com/archeopternix/markdowner/internal/fsutils"
)

type VTTmporter struct{}

func New() *VTTmporter { return &VTTmporter{} }

func (*VTTmporter) Name() string { return "vtt" }

func (*VTTmporter) Accept(ctx context.Context, src md.ImportSource) bool {
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

// Import reads the markdown source, extracts frontmatter if present, and saves a md.Document to the store.
func (*VTTmporter) Import(ctx context.Context, store md.DocumentStore, src md.ImportSource) (*md.Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if store == nil {
		return nil, fmt.Errorf("store is nil")
	}
	if src.Reader == nil {
		return nil, fmt.Errorf("source reader is nil")
	}

	_ = ctx
	b, err := io.ReadAll(src.Reader)
	if err != nil {
		return nil, err
	}

	bodyText := string(b)

	fmap, err := format.DecodeYAML([]byte(bodyText))
	if err != nil {
		return nil, err
	}
	fm := md.DecodeFrontmatterFromMap(fmap)

	now := time.Now().UTC()
	t := src.ModTime
	if t.IsZero() {
		t = now
	}
	stamp := t.Format("02.01.2006 15:04")

	// Defaults if missing or no frontmatter.
	if strings.TrimSpace(fm.Title) == "" {
		fm.Title = "Meeting minutes: " + fsutils.FileBaseNameNoExt(src.Name)
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

	slog.Debug("Parsed VTT", "title", fm.Title, "date", fm.Date)

	bodyText, speakers := extractVTTBodyText(bodyText)
	if strings.TrimSpace(bodyText) == "" {
		// Create a minimal markdown document if none is present.
		// Keep it simple and deterministic.
		bodyText = "# " + fm.Title + "\n"
	}
	if len(speakers) > 0 {
		var b strings.Builder
		b.WriteString("# " + fm.Title + "\n")
		b.WriteString("## Participants:\n")
		for _, speaker := range speakers {
			b.WriteString("* " + html.UnescapeString(speaker))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString("## Conversation:\n")
		b.WriteString(bodyText)
		bodyText = b.String()
	}

	doc := &md.Document{
		Frontmatter: fm,
		Markdown:    bodyText,
		Media:       nil,
	}

	if err := store.SaveOrUpdate(ctx, doc); err != nil {
		return nil, err
	}
	return doc, nil
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
