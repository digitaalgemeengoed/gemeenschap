package main

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// maakZip: een zip met deze delen (naam → inhoud), zoals Office en OpenDocument.
func maakZip(t *testing.T, naam string, delen map[string]string) *os.File {
	t.Helper()
	p := filepath.Join(t.TempDir(), naam)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for n, inhoud := range delen {
		w, _ := z.Create(n)
		w.Write([]byte(inhoud))
	}
	z.Close()
	f.Seek(0, 0)
	return f
}

func leesTekst(t *testing.T, f *os.File, naam string) string {
	t.Helper()
	st, _ := f.Stat()
	tekst, err := haalTekst(context.Background(), f, st.Size(), naam, "")
	if err != nil {
		t.Fatalf("%s: %v", naam, err)
	}
	return tekst
}

func TestTekstWord(t *testing.T) {
	f := maakZip(t, "a.docx", map[string]string{
		"word/document.xml": `<w:document xmlns:w="w"><w:body><w:p><w:r><w:t>Verslag van de </w:t></w:r><w:r><w:t>kerngroep</w:t></w:r></w:p>` +
			`<w:p><w:r><w:fldChar/><w:instrText>PAGEREF _Toc1</w:instrText><w:t>Archivering</w:t></w:r></w:p></w:body></w:document>`,
		"word/footer1.xml": `<w:ftr xmlns:w="w"><w:p><w:r><w:t>Voettekst</w:t></w:r></w:p></w:ftr>`,
		"word/styles.xml":  `<w:styles xmlns:w="w"><w:t>NietDit</w:t></w:styles>`,
	})
	tekst := leesTekst(t, f, "a.docx")
	for _, moet := range []string{"Verslag van de kerngroep", "Archivering", "Voettekst"} {
		if !strings.Contains(tekst, moet) {
			t.Errorf("mist %q in %q", moet, tekst)
		}
	}
	for _, niet := range []string{"PAGEREF", "NietDit"} {
		if strings.Contains(tekst, niet) {
			t.Errorf("hoort er niet in: %q in %q", niet, tekst)
		}
	}
	if !strings.Contains(tekst, "kerngroep\nArchivering") {
		t.Errorf("alinea's niet gescheiden: %q", tekst)
	}
}

func TestTekstExcelEnPowerPoint(t *testing.T) {
	x := maakZip(t, "b.xlsx", map[string]string{
		"xl/sharedStrings.xml":     `<sst><si><t>Begroting</t></si><si><t>Zandvoort</t></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet><sheetData><row><c t="inlineStr"><is><t>Inline tekst</t></is></c><c><f>SUM(A1)</f><v>42</v></c></row></sheetData></worksheet>`,
	})
	tekst := leesTekst(t, x, "b.xlsx")
	for _, moet := range []string{"Begroting", "Zandvoort", "Inline tekst"} {
		if !strings.Contains(tekst, moet) {
			t.Errorf("xlsx mist %q in %q", moet, tekst)
		}
	}
	if strings.Contains(tekst, "SUM") {
		t.Errorf("formule in de tekst: %q", tekst)
	}
	p := maakZip(t, "c.pptx", map[string]string{
		"ppt/slides/slide10.xml":            `<p:sld xmlns:a="a" xmlns:p="p"><a:p><a:r><a:t>Tiende dia</a:t></a:r></a:p></p:sld>`,
		"ppt/slides/slide2.xml":             `<p:sld xmlns:a="a" xmlns:p="p"><a:p><a:r><a:t>Tweede dia</a:t></a:r></a:p></p:sld>`,
		"ppt/notesSlides/notesSlide2.xml":   `<p:notes xmlns:a="a" xmlns:p="p"><a:p><a:r><a:t>Sprekersnotitie</a:t></a:r></a:p></p:notes>`,
		"ppt/slideLayouts/slideLayout1.xml": `<x><a:t xmlns:a="a">Klik om te bewerken</a:t></x>`,
	})
	tekst = leesTekst(t, p, "c.pptx")
	if !strings.Contains(tekst, "Tweede dia") || !strings.Contains(tekst, "Sprekersnotitie") || strings.Contains(tekst, "Klik om") {
		t.Errorf("pptx: %q", tekst)
	}
	if strings.Index(tekst, "Tweede dia") > strings.Index(tekst, "Tiende dia") {
		t.Errorf("dia's in de verkeerde volgorde: %q", tekst)
	}
}

func TestTekstOpenDocument(t *testing.T) {
	f := maakZip(t, "d.odt", map[string]string{
		"content.xml": `<office:document-content xmlns:office="o" xmlns:text="t"><office:body><office:text>` +
			`<text:h>Plan van aanpak</text:h><text:p>Een<text:s/>pilot in Zandvoort</text:p></office:text></office:body></office:document-content>`,
		"meta.xml": `<office:document-meta xmlns:office="o" xmlns:dc="dc"><office:meta><dc:title>Titel uit meta</dc:title></office:meta></office:document-meta>`,
	})
	tekst := leesTekst(t, f, "d.odt")
	for _, moet := range []string{"Plan van aanpak", "Een pilot in Zandvoort", "Titel uit meta"} {
		if !strings.Contains(tekst, moet) {
			t.Errorf("odt mist %q in %q", moet, tekst)
		}
	}
}

func TestTekstGewoonEnHTML(t *testing.T) {
	p := filepath.Join(t.TempDir(), "e.txt")
	os.WriteFile(p, []byte("Co\xf6peratie overleg"), 0o600) // Latin-1
	f, _ := os.Open(p)
	if tekst := leesTekst(t, f, "e.txt"); tekst != "Coöperatie overleg" {
		t.Errorf("latin-1: %q", tekst)
	}
	if got := netjes(htmlNaarTekst(`<p>Een &amp; twee</p><script>weg()</script>`)); got != "Een & twee" {
		t.Errorf("html: %q", got)
	}
	if _, err := haalTekst(context.Background(), f, 0, "foto.jpg", "image/jpeg"); err != errGeenTekst {
		t.Errorf("jpg: %v", err)
	}
}

func TestNetjesKapAf(t *testing.T) {
	lang := strings.Repeat("woord ", maxTekst)
	if n := len([]rune(netjes(lang))); n > maxTekst {
		t.Errorf("niet afgekapt: %d", n)
	}
}
