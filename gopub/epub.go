package gopub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path"

	"golang.org/x/net/html/charset"
)

// Reader represents a readable epub file.
type Reader struct {
	Container
	files map[string]*zip.File
}

// ReadCloser represents a readable epub file that can be closed.
type ReadCloser struct {
	Reader
	F_SIZE int64
	f      *os.File
}

// OpenReader opens the epub file at name and returns a ReadCloser.
func OpenReader(name string) (*ReadCloser, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	rc := &ReadCloser{f: f, F_SIZE: fi.Size()}
	z, err := zip.NewReader(f, fi.Size())
	if err != nil {
		f.Close()
		return nil, err
	}

	if err = rc.init(z); err != nil {
		f.Close()
		return nil, err
	}

	return rc, nil
}

// NewReader reads an epub from ra. The caller retains ownership of ra.
func NewReader(ra io.ReaderAt, size int64) (*Reader, error) {
	z, err := zip.NewReader(ra, size)
	if err != nil {
		return nil, err
	}

	r := new(Reader)
	if err = r.init(z); err != nil {
		return nil, err
	}
	return r, nil
}

// Close closes the epub file.
func (rc *ReadCloser) Close() {
	if rc.f != nil {
		rc.f.Close()
	}
}

func (r *Reader) init(z *zip.Reader) error {
	r.files = make(map[string]*zip.File)
	for _, f := range z.File {
		r.files[f.Name] = f
	}

	if err := r.setContainer(); err != nil {
		return err
	}
	if err := r.setPackages(); err != nil {
		return err
	}
	if err := r.setItems(); err != nil {
		return err
	}
	if err := r.setNCX(); err != nil {
		return err
	}
	return r.setTOC()
}

func (r *Reader) setContainer() error {
	containerZipFile, ok := r.files[containerPath]
	if !ok {
		return ErrNoContainerfile
	}

	/*
		if containerZipFile == nil {
			return ErrBadContainerfile
		}
	*/
	f, err := containerZipFile.Open()
	if err != nil {
		return ErrBadContainerfile
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	if err := xmlDecodeBytes(data, &r.Container); err != nil {
		return err
	}

	if len(r.Container.Rootfiles) < 1 {
		return ErrNoRootfile
	}
	return nil
}

func (r *Reader) setPackages() error {
	for _, rf := range r.Container.Rootfiles {
		zf := r.files[rf.FullPath]
		if zf == nil {
			return ErrBadRootfile
		}

		f, err := zf.Open()
		if err != nil {
			return err
		}

		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return err
		}

		if err := xmlDecodeBytes(data, &rf.Package); err != nil {
			return err
		}

		// Cover from manifest properties (EPUB 3.0).
		for _, manifestItem := range rf.Manifest.Items {
			if manifestItem.Properties == "cover-image" {
				rf.Metadata.CoverManifestId = manifestItem.ID
				break
			}
		}

		processRefinements(&rf.Metadata)
	}
	return nil
}

func (r *Reader) setItems() error {
	itemrefCount := 0
	for _, rf := range r.Container.Rootfiles {
		itemMap := make(map[string]*ManifestItem)
		for i := range rf.Manifest.Items {
			item := &rf.Manifest.Items[i]
			itemMap[item.ID] = item
			abs := path.Join(path.Dir(rf.FullPath), item.HREF)
			item.F = r.files[abs]
		}

		for i := range rf.Spine.Itemrefs {
			itemref := &rf.Spine.Itemrefs[i]
			itemref.ManifestItem = itemMap[itemref.IDREF]
			if itemref.ManifestItem == nil {
				return ErrBadItemref
			}
		}
		itemrefCount += len(rf.Spine.Itemrefs)
	}

	if itemrefCount < 1 {
		return ErrNoItemref
	}
	return nil
}

// xmlDecodeBytes strips a UTF-8 BOM and decodes XML with charset support.
func xmlDecodeBytes(data []byte, v any) error {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	data = escapeInvalidAmpersands(data)
	data = escapeNonAsciiTags(data)
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.CharsetReader = charset.NewReaderLabel
	return dec.Decode(v)
}

// escapeNonAsciiTags replaces '<' that start a tag whose name begins with a
// non-ASCII byte (e.g. CJK characters) with '&lt;'. Such sequences are never
// valid epub XML element names but appear as unescaped text in some malformed
// epubs (e.g. "<物語>" in an NCX docTitle).
func escapeNonAsciiTags(data []byte) []byte {
	if !bytes.Contains(data, []byte("<")) {
		return data
	}
	var buf bytes.Buffer
	buf.Grow(len(data))
	for i := 0; i < len(data); i++ {
		if data[i] != '<' {
			buf.WriteByte(data[i])
			continue
		}
		// Check for </X where X is non-ASCII.
		if i+2 < len(data) && data[i+1] == '/' && data[i+2] >= 0x80 {
			buf.WriteString("&lt;")
			continue
		}
		// Check for <X where X is non-ASCII.
		if i+1 < len(data) && data[i+1] >= 0x80 {
			buf.WriteString("&lt;")
			continue
		}
		buf.WriteByte('<')
	}
	return buf.Bytes()
}

// escapeInvalidAmpersands replaces bare & that aren't part of a valid XML
// entity or character reference with &amp;. Handles malformed epub XML.
func escapeInvalidAmpersands(data []byte) []byte {
	if !bytes.Contains(data, []byte("&")) {
		return data
	}
	var buf bytes.Buffer
	buf.Grow(len(data))
	i := 0
	for i < len(data) {
		if data[i] != '&' {
			buf.WriteByte(data[i])
			i++
			continue
		}
		if end := validEntityEnd(data, i); end > i {
			buf.Write(data[i:end])
			i = end
		} else {
			buf.WriteString("&amp;")
			i++
		}
	}
	return buf.Bytes()
}

// validEntityEnd returns the index past the closing ';' of a valid XML entity
// or character reference starting at data[i] (which must be '&'), or 0.
func validEntityEnd(data []byte, i int) int {
	j := i + 1
	if j >= len(data) {
		return 0
	}
	if data[j] == '#' {
		j++
		if j >= len(data) {
			return 0
		}
		if data[j] == 'x' || data[j] == 'X' {
			j++
			start := j
			for j < len(data) && isHexDigit(data[j]) {
				j++
			}
			if j == start || j >= len(data) || data[j] != ';' {
				return 0
			}
			return j + 1
		}
		start := j
		for j < len(data) && data[j] >= '0' && data[j] <= '9' {
			j++
		}
		if j == start || j >= len(data) || data[j] != ';' {
			return 0
		}
		return j + 1
	}
	if !isNameStartByte(data[j]) {
		return 0
	}
	j++
	for j < len(data) && isNameByte(data[j]) {
		j++
	}
	if j >= len(data) || data[j] != ';' {
		return 0
	}
	return j + 1
}

func isNameStartByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == ':'
}

func isNameByte(c byte) bool {
	return isNameStartByte(c) || (c >= '0' && c <= '9') || c == '-' || c == '.'
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
