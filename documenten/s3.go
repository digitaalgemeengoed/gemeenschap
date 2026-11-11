package main

// Een kleine S3-client (AWS Signature Version 4, path-style) voor Garage:
// alleen wat de dienst nodig heeft. Objecten worden nooit overschreven.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

type S3 struct {
	Eindpunt string // http://garage:3900
	Regio    string // garage
	Bucket   string
	Sleutel  string
	Geheim   string
	HTTP     *http.Client
}

var errNietGevonden = errors.New("niet gevonden")

const legeHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// codeerPad codeert een objectpad zoals SigV4 dat wil (RFC 3986, '/' blijft).
func codeerPad(p string) string {
	var b strings.Builder
	for _, c := range []byte(p) {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' || c == '/' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func codeerWaarde(s string) string {
	return strings.ReplaceAll(codeerPad(s), "/", "%2F")
}

func hmacSHA256(sleutel []byte, data string) []byte {
	h := hmac.New(sha256.New, sleutel)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// verzoek bouwt en ondertekent een aanvraag. payloadHash is de SHA-256 (hex)
// van de body, of legeHash zonder body.
func (s *S3) verzoek(ctx context.Context, methode, sleutel string, query url.Values, body io.Reader, lengte int64, payloadHash string, extra http.Header) (*http.Request, error) {
	pad := "/" + s.Bucket
	if sleutel != "" {
		pad += "/" + sleutel
	}
	u, err := url.Parse(s.Eindpunt)
	if err != nil {
		return nil, err
	}
	canonPad := codeerPad(pad)
	// Query canoniek: gesorteerd, beide kanten gecodeerd.
	var delen []string
	for k, vs := range query {
		for _, v := range vs {
			delen = append(delen, codeerWaarde(k)+"="+codeerWaarde(v))
		}
	}
	sort.Strings(delen)
	canonQuery := strings.Join(delen, "&")
	volledig := strings.TrimRight(s.Eindpunt, "/") + canonPad
	if canonQuery != "" {
		volledig += "?" + canonQuery
	}
	req, err := http.NewRequestWithContext(ctx, methode, volledig, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.ContentLength = lengte
	}
	for k, vs := range extra {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	nu := time.Now().UTC()
	amzDatum := nu.Format("20060102T150405Z")
	dag := nu.Format("20060102")
	req.Header.Set("x-amz-date", amzDatum)
	req.Header.Set("x-amz-content-sha256", payloadHash)

	koppen := map[string]string{"host": u.Host, "x-amz-date": amzDatum, "x-amz-content-sha256": payloadHash}
	for k := range extra {
		koppen[strings.ToLower(k)] = strings.TrimSpace(extra.Get(k))
	}
	namen := make([]string, 0, len(koppen))
	for k := range koppen {
		namen = append(namen, k)
	}
	sort.Strings(namen)
	var canonKoppen strings.Builder
	for _, k := range namen {
		canonKoppen.WriteString(k + ":" + koppen[k] + "\n")
	}
	ondertekend := strings.Join(namen, ";")
	canon := strings.Join([]string{methode, canonPad, canonQuery, canonKoppen.String(), ondertekend, payloadHash}, "\n")
	h := sha256.Sum256([]byte(canon))
	bereik := dag + "/" + s.Regio + "/s3/aws4_request"
	teOndertekenen := "AWS4-HMAC-SHA256\n" + amzDatum + "\n" + bereik + "\n" + hex.EncodeToString(h[:])
	k := hmacSHA256([]byte("AWS4"+s.Geheim), dag)
	k = hmacSHA256(k, s.Regio)
	k = hmacSHA256(k, "s3")
	k = hmacSHA256(k, "aws4_request")
	handtekening := hex.EncodeToString(hmacSHA256(k, teOndertekenen))
	req.Header.Set("Authorization", fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.Sleutel, bereik, ondertekend, handtekening))
	return req, nil
}

func s3Fout(resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errNietGevonden
	}
	return fmt.Errorf("S3 %s: %s", resp.Status, strings.TrimSpace(string(b)))
}

// Zet uploadt een bestand. sha256Hex is de hash van de inhoud; S3 controleert hem.
func (s *S3) Zet(ctx context.Context, sleutel string, f *os.File, lengte int64, sha256Hex, soort string) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	extra := http.Header{}
	extra.Set("Content-Type", soort)
	req, err := s.verzoek(ctx, http.MethodPut, sleutel, nil, io.NopCloser(f), lengte, sha256Hex, extra)
	if err != nil {
		return err
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return s3Fout(resp)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

// Haal geeft de inhoud van een object; de aanroeper sluit hem.
func (s *S3) Haal(ctx context.Context, sleutel string) (io.ReadCloser, int64, error) {
	req, err := s.verzoek(ctx, http.MethodGet, sleutel, nil, nil, 0, legeHash, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, 0, s3Fout(resp)
	}
	return resp.Body, resp.ContentLength, nil
}

// HaalVanaf: de inhoud vanaf een positie (voor Range-verzoeken).
func (s *S3) HaalVanaf(ctx context.Context, sleutel string, vanaf int64) (io.ReadCloser, error) {
	extra := http.Header{}
	if vanaf > 0 {
		extra.Set("Range", fmt.Sprintf("bytes=%d-", vanaf))
	}
	req, err := s.verzoek(ctx, http.MethodGet, sleutel, nil, nil, 0, legeHash, extra)
	if err != nil {
		return nil, err
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, s3Fout(resp)
	}
	return resp.Body, nil
}

// Bestaat: of een object er is.
func (s *S3) Bestaat(ctx context.Context, sleutel string) (bool, error) {
	req, err := s.verzoek(ctx, http.MethodHead, sleutel, nil, nil, 0, legeHash, nil)
	if err != nil {
		return false, err
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return false, err
	}
	resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusOK:
		return true, nil
	case resp.StatusCode == http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("S3 HEAD %s: %s", sleutel, resp.Status)
	}
}

type s3Lijst struct {
	Inhoud []struct {
		Key  string `xml:"Key"`
		Size int64  `xml:"Size"`
	} `xml:"Contents"`
	Afgekapt bool   `xml:"IsTruncated"`
	Volgende string `xml:"NextContinuationToken"`
}

// Lijst geeft alle objecten met dit voorvoegsel (sleutel → grootte).
func (s *S3) Lijst(ctx context.Context, voorvoegsel string) (map[string]int64, error) {
	uit := map[string]int64{}
	token := ""
	for {
		q := url.Values{"list-type": {"2"}, "prefix": {voorvoegsel}, "max-keys": {"1000"}}
		if token != "" {
			q.Set("continuation-token", token)
		}
		req, err := s.verzoek(ctx, http.MethodGet, "", q, nil, 0, legeHash, nil)
		if err != nil {
			return nil, err
		}
		resp, err := s.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			return nil, s3Fout(resp)
		}
		var l s3Lijst
		err = xml.NewDecoder(resp.Body).Decode(&l)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		for _, o := range l.Inhoud {
			uit[o.Key] = o.Size
		}
		if !l.Afgekapt || l.Volgende == "" {
			return uit, nil
		}
		token = l.Volgende
	}
}

// Klaar: antwoordt de opslag en bestaat de bucket?
func (s *S3) Klaar(ctx context.Context) error {
	req, err := s.verzoek(ctx, http.MethodHead, "", nil, nil, 0, legeHash, nil)
	if err != nil {
		return err
	}
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("bucket " + s.Bucket + ": " + resp.Status)
	}
	return nil
}
