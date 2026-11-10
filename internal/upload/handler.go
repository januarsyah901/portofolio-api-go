package upload

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/januarsyah901/portofolio-api-go/internal/httpx"
)

// Messages identical to src/routes/upload.ts / multer.
const (
	msgImageType  = "Hanya file gambar (.jpg, .jpeg, .png, .gif, .webp) yang diperbolehkan."
	msgCVType     = "Hanya file PDF yang diperbolehkan untuk CV."
	msgNoImage    = "Tidak ada file yang diunggah."
	msgNoCV       = "Tidak ada file CV yang diunggah."
	msgTooLarge   = "File too large"
	msgUnexpected = "Unexpected field"
	msgBadForm    = "Permintaan multipart tidak valid."
	msgCVOK       = "CV berhasil diperbarui."
)

const (
	maxImage = 5 << 20
	maxCV    = 10 << 20
	// multipartSlack is allowed on top of the file limit for boundaries, headers and small text fields.
	multipartSlack = 1 << 20
	sniffLen       = 512
)

// Handler serves uploads and the static files they produce.
type Handler struct {
	PublicDir string
}

// kind describes one upload endpoint.
type kind struct {
	field   string
	max     int64
	noFile  string
	badType string
	accepts func(ext, mimeType, sniffed string) bool
}

var imageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
var imageMimes = map[string]bool{"image/jpeg": true, "image/jpg": true, "image/png": true, "image/gif": true, "image/webp": true}
var sniffedImages = map[string]bool{"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true}

func (h *Handler) image(w http.ResponseWriter, r *http.Request) {
	h.receive(w, r, imageKind, func(f *incoming) (string, func(), error) {
		dir := filepath.Join(h.PublicDir, "assets", "uploads")
		var out *os.File
		var name string
		for range 5 {
			name = fmt.Sprintf("img-%d-%d%s", time.Now().UnixMilli(), rand.IntN(1_000_000_001), f.ext)
			var err error
			out, err = os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if err == nil {
				break
			}
			if !errors.Is(err, os.ErrExist) {
				return "", nil, err
			}
			out = nil
		}
		if out == nil {
			return "", nil, errors.New("could not pick a unique file name")
		}
		full := out.Name()
		undo := func() { _ = os.Remove(full) }
		if err := f.copyTo(out); err != nil {
			_ = out.Close()
			undo()
			return "", nil, err
		}
		if err := out.Close(); err != nil {
			undo()
			return "", nil, err
		}
		return name, undo, nil
	}, func(w http.ResponseWriter, _ *http.Request, name string) {
		httpx.JSON(w, http.StatusOK, map[string]string{"url": "/assets/uploads/" + name})
	})
}

func (h *Handler) cv(w http.ResponseWriter, r *http.Request) {
	var tmpName string
	h.receive(w, r, cvKind, func(f *incoming) (string, func(), error) {
		// Written to a temp file in the same directory; renamed into place only after the
		// whole request parsed cleanly, so a failed upload never clobbers the current CV.
		tmp, err := os.CreateTemp(h.PublicDir, ".cv-*.tmp")
		if err != nil {
			return "", nil, err
		}
		tmpName = tmp.Name()
		undo := func() { _ = os.Remove(tmpName) }
		if err := f.copyTo(tmp); err != nil {
			_ = tmp.Close()
			undo()
			return "", nil, err
		}
		if err := tmp.Chmod(0o644); err != nil {
			_ = tmp.Close()
			undo()
			return "", nil, err
		}
		if err := tmp.Close(); err != nil {
			undo()
			return "", nil, err
		}
		return tmpName, undo, nil
	}, func(w http.ResponseWriter, r *http.Request, _ string) {
		if err := os.Rename(tmpName, filepath.Join(h.PublicDir, "cv.pdf")); err != nil {
			_ = os.Remove(tmpName)
			httpx.ServerError(w, r, "saving cv", err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"url": "/cv.pdf", "message": msgCVOK})
	})
}

var imageKind = kind{
	field: "image", max: maxImage, noFile: msgNoImage, badType: msgImageType,
	accepts: func(ext, mt, sniffed string) bool {
		return imageExts[ext] && imageMimes[mt] && sniffedImages[sniffed]
	},
}

var cvKind = kind{
	field: "cv", max: maxCV, noFile: msgNoCV, badType: msgCVType,
	accepts: func(ext, mt, sniffed string) bool {
		return ext == ".pdf" && mt == "application/pdf" && sniffed == "application/pdf"
	},
}

// incoming is the file part being stored.
type incoming struct {
	ext  string
	head []byte // first bytes already read for sniffing
	part io.Reader
	max  int64
}

// copyTo writes head+rest of the part to dst, failing with errTooLarge past the limit.
func (f *incoming) copyTo(dst io.Writer) error {
	src := io.MultiReader(bytes.NewReader(f.head), f.part)
	n, err := io.Copy(dst, io.LimitReader(src, f.max+1))
	if err != nil {
		return err
	}
	if n > f.max {
		return errTooLarge
	}
	return nil
}

var errTooLarge = errors.New(msgTooLarge)

// receive parses the multipart body, validates the single expected file part, stores it with
// store, and calls done on success. Any failure removes what was stored and answers 400.
func (h *Handler) receive(w http.ResponseWriter, r *http.Request, k kind,
	store func(f *incoming) (string, func(), error), done func(w http.ResponseWriter, r *http.Request, name string)) {

	// Non-multipart requests: multer skips them, leaving req.file unset.
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if !strings.HasPrefix(mediaType, "multipart/") {
		httpx.Error(w, http.StatusBadRequest, k.noFile)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, k.max+multipartSlack)
	mr, err := r.MultipartReader()
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, msgBadForm)
		return
	}

	var (
		stored string
		undo   func()
		have   bool
	)
	fail := func(status int, msg string) {
		if undo != nil {
			undo()
		}
		httpx.Error(w, status, msg)
	}

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			fail(http.StatusBadRequest, readErrMsg(err))
			return
		}
		if part.FileName() == "" { // plain form field: ignore (bounded by the request limit)
			if _, err := io.Copy(io.Discard, part); err != nil {
				fail(http.StatusBadRequest, readErrMsg(err))
				return
			}
			continue
		}
		if part.FormName() != k.field || have {
			fail(http.StatusBadRequest, msgUnexpected)
			return
		}

		// Only the extension of the client file name is used; the stored name is ours.
		ext := strings.ToLower(path.Ext(strings.ReplaceAll(part.FileName(), `\`, "/")))
		mt, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		head := make([]byte, sniffLen)
		n, err := io.ReadFull(part, head)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			fail(http.StatusBadRequest, readErrMsg(err))
			return
		}
		head = head[:n]
		if !k.accepts(ext, strings.ToLower(mt), sniffMIME(head)) {
			fail(http.StatusBadRequest, k.badType)
			return
		}
		stored, undo, err = store(&incoming{ext: ext, head: head, part: part, max: k.max})
		if err != nil {
			undo = nil // store cleans up after itself on error
			if errors.Is(err, errTooLarge) || isTooBig(err) {
				httpx.Error(w, http.StatusBadRequest, msgTooLarge)
				return
			}
			httpx.ServerError(w, r, "saving upload", err)
			return
		}
		have = true
	}
	if !have {
		httpx.Error(w, http.StatusBadRequest, k.noFile)
		return
	}
	done(w, r, stored)
}

func isTooBig(err error) bool {
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}

func readErrMsg(err error) string {
	if isTooBig(err) {
		return msgTooLarge
	}
	return msgBadForm
}

func sniffMIME(head []byte) string {
	ct := http.DetectContentType(head)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return ct
}
