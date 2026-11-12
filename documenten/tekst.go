package main

// De tekst uit een document, voor de zoekindex (zoeken.go). Zonder diensten
// van buiten: Word, Excel en PowerPoint (OOXML) en OpenDocument zijn zip met
// XML; die leest de dienst zelf. Pdf gaat met pdftotext (poppler, in het
// image). Andere soorten: alleen de naam en de kenmerken zijn doorzoekbaar.
//
// Grenzen tegen zip-bommen en reuzenbestanden: hooguit maxTekstBron bytes
// lezen, hooguit maxTekst tekens bewaren.

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxTekstBron = 64 << 20 // zoveel lezen we hooguit van een bestand (en uitgepakt per zip)
	maxTekst     = 400_000  // zoveel tekens gaan hooguit in de index
)

var errGeenTekst = errors.New("geen tekst uit deze soort")

// tekstSoort: kunnen we hier tekst uit halen, en hoe?
func tekstSoort(naam, mediatype string) string {
	ext := strings.ToLower(path.Ext(naam))
	switch ext {
	case ".docx", ".docm", ".dotx", ".xlsx", ".xlsm", ".pptx", ".pptm", ".ppsx":
		return "ooxml"
	case ".odt", ".ods", ".odp", ".ott", ".odg":
		return "odf"
	case ".pdf":
		return "pdf"
	case ".txt", ".md", ".csv", ".tsv", ".json", ".xml", ".archimate", ".yaml", ".yml", ".log", ".ini", ".rtf":
		return "tekst"
	case ".html", ".htm":
		return "html"
	}
	switch {
	case mediatype == "application/pdf":
		return "pdf"
	case strings.HasPrefix(mediatype, "text/html"):
		return "html"
	case strings.HasPrefix(mediatype, "text/"):
		return "tekst"
	}
	return ""
}

// haalTekst: de tekst uit een bestand (f staat aan het begin).
func haalTekst(ctx context.Context, f *os.File, grootte int64, naam, mediatype string) (string, error) {
	var t string
	var err error
	switch tekstSoort(naam, mediatype) {
	case "ooxml":
		t, err = tekstOOXML(f, grootte)
	case "odf":
		t, err = tekstODF(f, grootte)
	case "pdf":
		t, err = tekstPDF(ctx, f.Name())
	case "tekst":
		t, err = tekstGewoon(f)
	case "html":
		var b []byte
		b, err = io.ReadAll(io.LimitReader(f, maxTekstBron))
		t = htmlNaarTekst(string(b))
	default:
		return "", errGeenTekst
	}
	return netjes(t), err
}

func tekstGewoon(r io.Reader) (string, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxTekstBron))
	if err != nil {
		return "", err
	}
	if !utf8.Valid(b) {
		// Waarschijnlijk Windows-1252 of Latin-1: elk byte als teken.
		var s strings.Builder
		for _, c := range b {
			s.WriteRune(rune(c))
		}
		return s.String(), nil
	}
	return string(b), nil
}

var htmlTag = regexp.MustCompile(`(?s)<script.*?</script>|<style.*?</style>|<[^>]*>`)

func htmlNaarTekst(s string) string {
	s = htmlTag.ReplaceAllString(s, " ")
	for oud, nieuw := range map[string]string{"&nbsp;": " ", "&amp;": "&", "&lt;": "<", "&gt;": ">", "&quot;": `"`, "&#39;": "'"} {
		s = strings.ReplaceAll(s, oud, nieuw)
	}
	return s
}

// netjes: witruimte samenvoegen (regels blijven), en afkappen.
func netjes(s string) string {
	var uit strings.Builder
	leeg, regel := 0, false
	for _, r := range s {
		switch {
		case r == '\n':
			if regel {
				uit.WriteByte('\n')
			}
			regel, leeg = false, 0
		case r == ' ' || r == '\t' || r == '\r' || r == ' ' || r < 32:
			leeg++
		default:
			if leeg > 0 && regel {
				uit.WriteByte(' ')
			}
			leeg, regel = 0, true
			uit.WriteRune(r)
		}
		if uit.Len() > maxTekst*4 {
			break
		}
	}
	t := strings.TrimSpace(uit.String())
	if utf8.RuneCountInString(t) > maxTekst {
		t = string([]rune(t)[:maxTekst])
	}
	return t
}

// --- Zip met XML -------------------------------------------------------------------

// Na deze elementen een nieuwe regel (alinea's, cellen, dia's).
var xmlRegel = map[string]bool{"p": true, "h": true, "tr": true, "row": true, "si": true, "tc": true, "c": true,
	"br": true, "tab": true, "cr": true, "table-cell": true, "table-row": true, "list-item": true, "line-break": true}

// Elementen waarvan de tekst niet meetelt (formules, verborgen opmaak).
var xmlOverslaan = map[string]bool{"f": true, "instrText": true}

// In de kenmerken (docProps/core.xml, meta.xml) alleen deze; niet het
// programma, de taal of het revisienummer.
var xmlKenmerken = map[string]bool{"title": true, "subject": true, "keyword": true, "keywords": true, "description": true}

func xmlTekst(r io.Reader, uit *strings.Builder, alleenKenmerken bool) error {
	d := xml.NewDecoder(io.LimitReader(r, maxTekstBron))
	d.Strict = false
	overslaan, kenmerk := 0, 0
	for {
		t, err := d.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch e := t.(type) {
		case xml.StartElement:
			if xmlOverslaan[e.Name.Local] {
				overslaan++
			}
			if xmlKenmerken[e.Name.Local] {
				kenmerk++
			}
			if e.Name.Local == "s" && e.Name.Space != "" { // ODF: <text:s/> is een spatie
				uit.WriteByte(' ')
			}
		case xml.EndElement:
			if xmlOverslaan[e.Name.Local] && overslaan > 0 {
				overslaan--
			}
			if xmlKenmerken[e.Name.Local] && kenmerk > 0 {
				kenmerk--
				if alleenKenmerken {
					uit.WriteByte('\n')
				}
			}
			if xmlRegel[e.Name.Local] {
				uit.WriteByte('\n')
			}
		case xml.CharData:
			if overslaan == 0 && (!alleenKenmerken || kenmerk > 0) {
				uit.Write(e)
			}
		}
		if uit.Len() > maxTekst*4 {
			return nil
		}
	}
}

func openZip(f *os.File, grootte int64) (*zip.Reader, error) {
	return zip.NewReader(f, grootte)
}

func leesZipDeel(zf *zip.File, uit *strings.Builder) error {
	if zf.UncompressedSize64 > maxTekstBron {
		return nil // te groot: overslaan
	}
	r, err := zf.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	return xmlTekst(r, uit, zf.Name == "docProps/core.xml" || zf.Name == "meta.xml")
}

// natuurlijk: slide2 vóór slide10.
var cijfers = regexp.MustCompile(`\d+`)

func natuurlijk(a, b string) bool {
	na, nb := cijfers.FindString(a), cijfers.FindString(b)
	if len(na) != len(nb) {
		return len(na) < len(nb)
	}
	if na != nb {
		return na < nb
	}
	return a < b
}

func tekstOOXML(f *os.File, grootte int64) (string, error) {
	z, err := openZip(f, grootte)
	if err != nil {
		return "", err
	}
	var delen []*zip.File
	for _, zf := range z.File {
		n := zf.Name
		switch {
		case n == "word/document.xml", strings.HasPrefix(n, "word/header"), strings.HasPrefix(n, "word/footer"),
			n == "word/footnotes.xml", n == "word/endnotes.xml", n == "word/comments.xml",
			n == "xl/sharedStrings.xml", strings.HasPrefix(n, "xl/worksheets/sheet"),
			strings.HasPrefix(n, "ppt/slides/slide"), strings.HasPrefix(n, "ppt/notesSlides/notesSlide"),
			n == "docProps/core.xml":
			if strings.HasSuffix(n, ".xml") {
				delen = append(delen, zf)
			}
		}
	}
	sort.SliceStable(delen, func(i, j int) bool { return natuurlijk(delen[i].Name, delen[j].Name) })
	var uit strings.Builder
	for _, zf := range delen {
		if strings.HasPrefix(zf.Name, "xl/worksheets/") {
			// Alleen tekst die in het blad zelf staat (inlineStr); de rest staat in sharedStrings.
			if err := leesBladTekst(zf, &uit); err != nil {
				return uit.String(), err
			}
			continue
		}
		if err := leesZipDeel(zf, &uit); err != nil {
			return uit.String(), err
		}
		uit.WriteByte('\n')
	}
	return uit.String(), nil
}

// leesBladTekst: in een werkblad alleen <is><t>…</t></is> (tekst in de cel zelf).
func leesBladTekst(zf *zip.File, uit *strings.Builder) error {
	if zf.UncompressedSize64 > maxTekstBron {
		return nil
	}
	r, err := zf.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	d := xml.NewDecoder(io.LimitReader(r, maxTekstBron))
	in := 0
	for {
		t, err := d.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch e := t.(type) {
		case xml.StartElement:
			if e.Name.Local == "is" {
				in++
			}
		case xml.EndElement:
			if e.Name.Local == "is" && in > 0 {
				in--
				uit.WriteByte('\n')
			}
		case xml.CharData:
			if in > 0 {
				uit.Write(e)
			}
		}
	}
}

func tekstODF(f *os.File, grootte int64) (string, error) {
	z, err := openZip(f, grootte)
	if err != nil {
		return "", err
	}
	var uit strings.Builder
	for _, naam := range []string{"meta.xml", "content.xml"} {
		for _, zf := range z.File {
			if zf.Name == naam {
				if err := leesZipDeel(zf, &uit); err != nil {
					return uit.String(), err
				}
				uit.WriteByte('\n')
			}
		}
	}
	return uit.String(), nil
}

// --- Pdf -------------------------------------------------------------------------

func tekstPDF(ctx context.Context, bestand string) (string, error) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", errGeenTekst
	}
	ctx, stop := context.WithTimeout(ctx, 60*time.Second)
	defer stop()
	var uit, fout bytes.Buffer
	cmd := exec.CommandContext(ctx, "pdftotext", "-q", "-enc", "UTF-8", "-l", "500", bestand, "-")
	cmd.Stdout, cmd.Stderr = &grensSchrijver{w: &uit, over: maxTekstBron}, &fout
	if err := cmd.Run(); err != nil && uit.Len() == 0 {
		return "", err
	}
	return uit.String(), nil
}

type grensSchrijver struct {
	w    io.Writer
	over int64
}

func (g *grensSchrijver) Write(p []byte) (int, error) {
	if g.over <= 0 {
		return len(p), nil
	}
	n := int64(len(p))
	if n > g.over {
		p = p[:g.over]
	}
	g.over -= int64(len(p))
	g.w.Write(p)
	return int(n), nil
}
