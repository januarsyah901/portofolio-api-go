package upload

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/januarsyah901/portofolio-api-go/internal/app"
	"github.com/januarsyah901/portofolio-api-go/internal/auth"
	"github.com/januarsyah901/portofolio-api-go/internal/config"
)

var (
	pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)
	jpgBytes = append([]byte("\xff\xd8\xff\xe0"), bytes.Repeat([]byte{0}, 64)...)
	gifBytes = append([]byte("GIF89a"), bytes.Repeat([]byte{0}, 64)...)
	webpData = append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{0}, 32)...)
	pdfBytes = []byte("%PDF-1.4\n%fake\n")
)

type env struct {
	mux     *http.ServeMux
	dir     string
	token   string
	uploads string
}

func setup(t *testing.T) env {
	t.Helper()
	dir := t.TempDir()
	j := auth.NewJWT("secret-for-tests-secret-for-tests")
	tok, _ := j.Sign(1)
	mux := http.NewServeMux()
	Register(mux, app.Deps{Config: config.Config{PublicDir: dir}, Guards: auth.Guards{JWT: j}})
	return env{mux: mux, dir: dir, token: tok, uploads: filepath.Join(dir, "assets", "uploads")}
}

type filePart struct {
	field, filename, contentType string
	data                         []byte
}

func multipartBody(t *testing.T, parts ...filePart) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, p := range parts {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="`+p.field+`"; filename="`+p.filename+`"`)
		h.Set("Content-Type", p.contentType)
		pw, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pw.Write(p.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, mw.FormDataContentType()
}

func (e env) post(t *testing.T, path, token string, parts ...filePart) *httptest.ResponseRecorder {
	t.Helper()
	body, ct := multipartBody(t, parts...)
	req := httptest.NewRequest("POST", path, body)
	req.Header.Set("Content-Type", ct)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

func message(rec *httptest.ResponseRecorder) string {
	var m struct{ Message string }
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	return m.Message
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

func TestRegisterCreatesUploadsDir(t *testing.T) {
	e := setup(t)
	if fi, err := os.Stat(e.uploads); err != nil || !fi.IsDir() {
		t.Fatal("uploads dir not created")
	}
}

var imgName = regexp.MustCompile(`^img-\d{13}-\d{1,10}\.(png|jpg|jpeg|gif|webp)$`)

func TestImageUploadSuccess(t *testing.T) {
	e := setup(t)
	cases := []struct {
		filename, ct string
		data         []byte
		ext          string
	}{
		{"photo.png", "image/png", pngBytes, ".png"},
		{"photo.JPG", "image/jpeg", jpgBytes, ".jpg"},
		{"photo.jpeg", "image/jpeg", jpgBytes, ".jpeg"},
		{"anim.gif", "image/gif", gifBytes, ".gif"},
		{"pic.webp", "image/webp", webpData, ".webp"},
	}
	for _, c := range cases {
		rec := e.post(t, "/api/admin/upload/image", e.token, filePart{"image", c.filename, c.ct, c.data})
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", c.filename, rec.Code, rec.Body)
		}
		var resp struct{ URL string }
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		name := strings.TrimPrefix(resp.URL, "/assets/uploads/")
		if name == resp.URL || !imgName.MatchString(name) || !strings.HasSuffix(name, c.ext) {
			t.Errorf("%s: bad url %q", c.filename, resp.URL)
		}
		got, err := os.ReadFile(filepath.Join(e.uploads, name))
		if err != nil || !bytes.Equal(got, c.data) {
			t.Errorf("%s: stored content mismatch (%v)", c.filename, err)
		}
		// and it is served back
		req := httptest.NewRequest("GET", resp.URL, nil)
		r2 := httptest.NewRecorder()
		e.mux.ServeHTTP(r2, req)
		if r2.Code != 200 || !bytes.Equal(r2.Body.Bytes(), c.data) {
			t.Errorf("%s: serve %d", c.filename, r2.Code)
		}
	}
	if len(entries(t, e.uploads)) != len(cases) {
		t.Errorf("unexpected files: %v", entries(t, e.uploads))
	}
}

func TestImageUploadRequiresJWT(t *testing.T) {
	e := setup(t)
	if rec := e.post(t, "/api/admin/upload/image", "", filePart{"image", "a.png", "image/png", pngBytes}); rec.Code != 401 {
		t.Errorf("image: %d", rec.Code)
	}
	if rec := e.post(t, "/api/admin/upload/cv", "bad", filePart{"cv", "a.pdf", "application/pdf", pdfBytes}); rec.Code != 401 {
		t.Errorf("cv: %d", rec.Code)
	}
	if len(entries(t, e.uploads)) != 0 {
		t.Error("files written without auth")
	}
}

func TestImageUploadRejections(t *testing.T) {
	e := setup(t)
	const typeMsg = "Hanya file gambar (.jpg, .jpeg, .png, .gif, .webp) yang diperbolehkan."
	cases := []struct {
		name string
		part filePart
		msg  string
	}{
		{"bad extension", filePart{"image", "a.txt", "image/png", pngBytes}, typeMsg},
		{"no extension", filePart{"image", "a", "image/png", pngBytes}, typeMsg},
		{"svg", filePart{"image", "a.svg", "image/svg+xml", []byte("<svg/>")}, typeMsg},
		{"bad mimetype", filePart{"image", "a.png", "text/plain", pngBytes}, typeMsg},
		{"no mimetype", filePart{"image", "a.png", "", pngBytes}, typeMsg},
		{"content is not an image", filePart{"image", "a.png", "image/png", []byte("<?php echo 1; ?>")}, typeMsg},
		{"html disguised as jpg", filePart{"image", "a.jpg", "image/jpeg", []byte("<html><script>alert(1)</script>")}, typeMsg},
		{"pdf content as png", filePart{"image", "a.png", "image/png", pdfBytes}, typeMsg},
		{"empty file", filePart{"image", "a.png", "image/png", nil}, typeMsg},
		{"wrong field", filePart{"file", "a.png", "image/png", pngBytes}, "Unexpected field"},
	}
	for _, c := range cases {
		rec := e.post(t, "/api/admin/upload/image", e.token, c.part)
		if rec.Code != 400 || message(rec) != c.msg {
			t.Errorf("%s: %d %q", c.name, rec.Code, message(rec))
		}
	}
	if n := entries(t, e.uploads); len(n) != 0 {
		t.Errorf("rejected uploads left files: %v", n)
	}
}

func TestUploadNoFile(t *testing.T) {
	e := setup(t)
	// multipart with only a text field
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	must(t, mw.WriteField("title", "x"))
	must(t, mw.Close())
	for _, c := range []struct{ path, msg string }{
		{"/api/admin/upload/image", "Tidak ada file yang diunggah."},
		{"/api/admin/upload/cv", "Tidak ada file CV yang diunggah."},
	} {
		req := httptest.NewRequest("POST", c.path, bytes.NewReader(buf.Bytes()))
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+e.token)
		rec := httptest.NewRecorder()
		e.mux.ServeHTTP(rec, req)
		if rec.Code != 400 || message(rec) != c.msg {
			t.Errorf("multipart no file %s: %d %q", c.path, rec.Code, message(rec))
		}
		// not multipart at all
		req = httptest.NewRequest("POST", c.path, strings.NewReader(`{"a":1}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+e.token)
		rec = httptest.NewRecorder()
		e.mux.ServeHTTP(rec, req)
		if rec.Code != 400 || message(rec) != c.msg {
			t.Errorf("json body %s: %d %q", c.path, rec.Code, message(rec))
		}
	}
}

func TestImageTooLarge(t *testing.T) {
	e := setup(t)
	big := append(append([]byte{}, pngBytes...), bytes.Repeat([]byte{1}, 5<<20)...) // > 5 MB
	rec := e.post(t, "/api/admin/upload/image", e.token, filePart{"image", "big.png", "image/png", big})
	if rec.Code != 400 || message(rec) != "File too large" {
		t.Errorf("%d %q", rec.Code, message(rec))
	}
	if n := entries(t, e.uploads); len(n) != 0 {
		t.Errorf("oversize left files: %v", n)
	}
	// exactly the limit is fine
	ok := append(append([]byte{}, pngBytes...), bytes.Repeat([]byte{1}, 5<<20-len(pngBytes))...)
	rec = e.post(t, "/api/admin/upload/image", e.token, filePart{"image", "ok.png", "image/png", ok})
	if rec.Code != 200 {
		t.Errorf("5MB exactly: %d %s", rec.Code, rec.Body)
	}
}

func TestHugeRequestBounded(t *testing.T) {
	e := setup(t)
	huge := append(append([]byte{}, pngBytes...), bytes.Repeat([]byte{1}, 8<<20)...) // well past limit+slack
	rec := e.post(t, "/api/admin/upload/image", e.token, filePart{"image", "huge.png", "image/png", huge})
	if rec.Code != 400 || message(rec) != "File too large" {
		t.Errorf("%d %q", rec.Code, message(rec))
	}
	if n := entries(t, e.uploads); len(n) != 0 {
		t.Errorf("left files: %v", n)
	}
}

func TestFilenameCannotEscape(t *testing.T) {
	e := setup(t)
	for _, fn := range []string{"../../evil.png", `..\..\evil.png`, "/etc/passwd.png", "a/../../b.png", "x.png\x00.txt"} {
		rec := e.post(t, "/api/admin/upload/image", e.token, filePart{"image", fn, "image/png", pngBytes})
		if rec.Code == 200 {
			var resp struct{ URL string }
			_ = json.Unmarshal(rec.Body.Bytes(), &resp)
			if !imgName.MatchString(strings.TrimPrefix(resp.URL, "/assets/uploads/")) {
				t.Errorf("%q: url %q", fn, resp.URL)
			}
		}
	}
	// nothing outside uploads, and only generated names inside it
	for _, n := range entries(t, e.dir) {
		if n != "assets" {
			t.Errorf("stray entry in public dir: %s", n)
		}
	}
	for _, n := range entries(t, e.uploads) {
		if !imgName.MatchString(n) {
			t.Errorf("bad stored name %q", n)
		}
	}
	parent := filepath.Dir(e.dir)
	if _, err := os.Stat(filepath.Join(parent, "evil.png")); err == nil {
		t.Error("escaped the public dir")
	}
}

func TestCVUpload(t *testing.T) {
	e := setup(t)
	rec := e.post(t, "/api/admin/upload/cv", e.token, filePart{"cv", "My CV.PDF", "application/pdf", pdfBytes})
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var resp struct{ URL, Message string }
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.URL != "/cv.pdf" || resp.Message != "CV berhasil diperbarui." {
		t.Errorf("resp %s", rec.Body)
	}
	got, _ := os.ReadFile(filepath.Join(e.dir, "cv.pdf"))
	if !bytes.Equal(got, pdfBytes) {
		t.Error("cv content")
	}
	// replace
	newPDF := []byte("%PDF-1.7 newer")
	if rec = e.post(t, "/api/admin/upload/cv", e.token, filePart{"cv", "b.pdf", "application/pdf", newPDF}); rec.Code != 200 {
		t.Fatalf("replace %d", rec.Code)
	}
	got, _ = os.ReadFile(filepath.Join(e.dir, "cv.pdf"))
	if !bytes.Equal(got, newPDF) {
		t.Error("cv not replaced")
	}
	// no temp files left
	for _, n := range entries(t, e.dir) {
		if n != "cv.pdf" && n != "assets" {
			t.Errorf("leftover %s", n)
		}
	}
}

func TestCVRejectionsKeepExistingCV(t *testing.T) {
	e := setup(t)
	must(t, os.WriteFile(filepath.Join(e.dir, "cv.pdf"), pdfBytes, 0o644))
	const msg = "Hanya file PDF yang diperbolehkan untuk CV."
	cases := []struct {
		name string
		part filePart
		msg  string
	}{
		{"bad ext", filePart{"cv", "cv.docx", "application/pdf", pdfBytes}, msg},
		{"bad mimetype", filePart{"cv", "cv.pdf", "application/octet-stream", pdfBytes}, msg},
		{"not really a pdf", filePart{"cv", "cv.pdf", "application/pdf", []byte("MZ\x90 exe")}, msg},
		{"png as pdf", filePart{"cv", "cv.pdf", "application/pdf", pngBytes}, msg},
		{"wrong field", filePart{"image", "cv.pdf", "application/pdf", pdfBytes}, "Unexpected field"},
		{"too large", filePart{"cv", "cv.pdf", "application/pdf", append([]byte("%PDF-"), bytes.Repeat([]byte{1}, 10<<20)...)}, "File too large"},
	}
	for _, c := range cases {
		rec := e.post(t, "/api/admin/upload/cv", e.token, c.part)
		if rec.Code != 400 || message(rec) != c.msg {
			t.Errorf("%s: %d %q", c.name, rec.Code, message(rec))
		}
	}
	got, _ := os.ReadFile(filepath.Join(e.dir, "cv.pdf"))
	if !bytes.Equal(got, pdfBytes) {
		t.Error("existing cv was modified by a rejected upload")
	}
	for _, n := range entries(t, e.dir) {
		if n != "cv.pdf" && n != "assets" {
			t.Errorf("leftover temp %s", n)
		}
	}
}

func get(e env, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, httptest.NewRequest("GET", target, nil))
	return rec
}

func TestStaticServing(t *testing.T) {
	e := setup(t)
	must(t, os.WriteFile(filepath.Join(e.uploads, "a.png"), pngBytes, 0o644))
	must(t, os.WriteFile(filepath.Join(e.dir, "cv.pdf"), pdfBytes, 0o644))
	must(t, os.WriteFile(filepath.Join(e.dir, "secret.txt"), []byte("secret"), 0o644))
	must(t, os.MkdirAll(filepath.Join(e.uploads, "sub"), 0o755))
	must(t, os.WriteFile(filepath.Join(e.uploads, "sub", "b.png"), pngBytes, 0o644))

	rec := get(e, "/assets/uploads/a.png")
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), pngBytes) || rec.Header().Get("Content-Type") != "image/png" {
		t.Errorf("png: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	rec = get(e, "/cv.pdf")
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), pdfBytes) || rec.Header().Get("Content-Type") != "application/pdf" {
		t.Errorf("cv: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if get(e, "/assets/uploads/sub/b.png").Code != 200 {
		t.Error("nested file should be served")
	}

	// Missing → JSON 404
	for _, target := range []string{"/assets/uploads/nope.png"} {
		rec = get(e, target)
		if rec.Code != 404 || message(rec) != "Endpoint tidak ditemukan." {
			t.Errorf("%s: %d %q", target, rec.Code, rec.Body)
		}
	}
	// Directory listings disabled
	for _, target := range []string{"/assets/uploads/", "/assets/uploads/sub", "/assets/uploads/sub/"} {
		rec = get(e, target)
		if rec.Code != 404 || strings.Contains(rec.Body.String(), "a.png") {
			t.Errorf("listing %s: %d %s", target, rec.Code, rec.Body)
		}
	}
	// Traversal: nothing may leak the secret file, whatever status is chosen.
	for _, target := range []string{
		"/assets/uploads/../../secret.txt",
		"/assets/uploads/..%2f..%2fsecret.txt",
		"/assets/uploads/%2e%2e/%2e%2e/secret.txt",
		"/assets/uploads/..%2f..%2f..%2fetc/passwd",
		"/assets/uploads/sub/../../../secret.txt",
		"/assets/uploads/..%5c..%5csecret.txt",
		"/assets/uploads//secret.txt",
	} {
		rec = get(e, target)
		if rec.Code == 200 || rec.Body.String() == "secret" { // 3xx from the mux cleaning the path is fine
			t.Errorf("traversal %s: %d %q", target, rec.Code, rec.Body)
		}
	}
	// Calling the handler directly (no mux path cleaning) with a raw ".." value.
	h := &Handler{PublicDir: e.dir}
	req := httptest.NewRequest("GET", "/x", nil)
	req.SetPathValue("file", "../../secret.txt")
	rec = httptest.NewRecorder()
	h.serveUpload(rec, req)
	if rec.Code != 404 {
		t.Errorf("direct traversal: %d", rec.Code)
	}
	// Symlink pointing outside is not followed.
	if err := os.Symlink(filepath.Join(e.dir, "secret.txt"), filepath.Join(e.uploads, "link.txt")); err == nil {
		if rec = get(e, "/assets/uploads/link.txt"); rec.Code == 200 {
			t.Error("symlink escape served")
		}
	}
	// cv.pdf missing / a directory
	must(t, os.Remove(filepath.Join(e.dir, "cv.pdf")))
	if rec = get(e, "/cv.pdf"); rec.Code != 404 || message(rec) != "Endpoint tidak ditemukan." {
		t.Errorf("missing cv: %d", rec.Code)
	}
}

func TestStaticHeadAndMethods(t *testing.T) {
	e := setup(t)
	must(t, os.WriteFile(filepath.Join(e.dir, "cv.pdf"), pdfBytes, 0o644))
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, httptest.NewRequest("HEAD", "/cv.pdf", nil))
	if rec.Code != 200 || rec.Body.Len() != 0 {
		t.Errorf("head: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	e.mux.ServeHTTP(rec, httptest.NewRequest("POST", "/cv.pdf", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("post static: %d", rec.Code)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
