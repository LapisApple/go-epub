package gopub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	"golang.org/x/net/html/charset"
)

// ReaderOptions configures optional behaviour for Reader and ReadCloser.
// The zero value is valid and applies no restrictions.
type ReaderOptions struct {
	// MaxFileSize limits how many bytes are read from any single file inside
	// the EPUB ZIP. 0 means unlimited. Set this when processing untrusted
	// EPUBs to guard against ZIP-bomb / OOM attacks.
	MaxFileSize int64
}

// Reader represents a readable epub file.
type Reader struct {
	Container
	files map[string]*zip.File
	Size  int64
	opts  ReaderOptions
}

// ReadCloser represents a readable epub file that can be closed.
type ReadCloser struct {
	Reader
	f *os.File
}

// OpenReader opens the epub file at name and returns a ReadCloser.
func OpenReader(name string, opts ...ReaderOptions) (*ReadCloser, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	return NewReaderOwning(f)
}

// NewReader reads an epub from f. The gopub.ReadCloser gains ownership of f.
func NewReaderOwning(f *os.File, opts ...ReaderOptions) (*ReadCloser, error) {
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	rc := &ReadCloser{f: f, Reader: Reader{Size: fi.Size()}}
	if len(opts) > 0 {
		rc.opts = opts[0]
	}
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
func NewReader(ra io.ReaderAt, size int64, opts ...ReaderOptions) (*Reader, error) {
	z, err := zip.NewReader(ra, size)
	if err != nil {
		return nil, err
	}

	r := new(Reader)
	r.Size = size
	if len(opts) > 0 {
		r.opts = opts[0]
	}
	if err = r.init(z); err != nil {
		return nil, err
	}
	return r, nil
}

// Close closes the epub file.
func (rc *ReadCloser) Close() error {
	if rc.f != nil {
		return rc.f.Close()
	}
	return nil
}

// readAll reads from r, honouring MaxFileSize when set.
// Returns ErrFileTooLarge if the data exceeds the limit (not a silent truncation).
func (reader *Reader) readAll(r io.Reader) ([]byte, error) {
	if reader.opts.MaxFileSize > 0 {
		// Read one byte beyond the limit to detect oversized files.
		data, err := io.ReadAll(io.LimitReader(r, reader.opts.MaxFileSize+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > reader.opts.MaxFileSize {
			return nil, ErrFileTooLarge
		}
		return data, nil
	}
	return io.ReadAll(r)
}

// readZipFile opens a ZIP entry, reads it fully, and closes it.
func (reader *Reader) readZipFile(zf *zip.File) ([]byte, error) {
	f, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return reader.readAll(f)
}

// readItem opens a ManifestItem, reads it fully, and closes it.
func (reader *Reader) readItem(item *ManifestItem) ([]byte, error) {
	f, err := item.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return reader.readAll(f)
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

	data, err := r.readZipFile(containerZipFile)
	if err != nil {
		return ErrBadContainerfile
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

		data, err := r.readZipFile(zf)
		if err != nil {
			return err
		}

		if err := xmlDecodeBytes(data, &rf.Package); err != nil {
			return err
		}

		if ver := rf.Package.Version; ver != "" {
			major := strings.SplitN(ver, ".", 2)[0]
			if major != "2" && major != "3" {
				return fmt.Errorf("epub: unsupported version %q", ver)
			}
		}

		// Cover from manifest properties (EPUB 3.0).
		for _, manifestItem := range rf.Manifest.Items {
			if hasProperty(manifestItem.Properties, "cover-image") {
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
			href, _ := url.PathUnescape(item.HREF)
			abs := path.Join(path.Dir(rf.FullPath), href)
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
// CDATA sections are passed through unchanged.
func escapeNonAsciiTags(data []byte) []byte {
	if !bytes.Contains(data, []byte("<")) {
		return data
	}
	var buf bytes.Buffer
	buf.Grow(len(data))
	cdataStart := []byte("<![CDATA[")
	cdataEnd := []byte("]]>")
	for i := 0; i < len(data); {
		// Pass CDATA sections through unchanged.
		if bytes.HasPrefix(data[i:], cdataStart) {
			end := bytes.Index(data[i+len(cdataStart):], cdataEnd)
			if end >= 0 {
				end += i + len(cdataStart) + len(cdataEnd)
				buf.Write(data[i:end])
				i = end
				continue
			}
			// Unclosed CDATA — emit the rest as-is.
			buf.Write(data[i:])
			break
		}
		if data[i] != '<' {
			buf.WriteByte(data[i])
			i++
			continue
		}
		// Check for </X where X is non-ASCII.
		if i+2 < len(data) && data[i+1] == '/' && data[i+2] >= 0x80 {
			buf.WriteString("&lt;")
			i++
			continue
		}
		// Check for <X where X is non-ASCII.
		if i+1 < len(data) && data[i+1] >= 0x80 {
			buf.WriteString("&lt;")
			i++
			continue
		}
		buf.WriteByte('<')
		i++
	}
	return buf.Bytes()
}

// escapeInvalidAmpersands replaces bare & that aren't part of a valid XML
// entity or character reference with &amp;. Handles malformed epub XML.
// CDATA sections are passed through unchanged (same as escapeNonAsciiTags).
func escapeInvalidAmpersands(data []byte) []byte {
	if !bytes.Contains(data, []byte("&")) {
		return data
	}
	var buf bytes.Buffer
	buf.Grow(len(data))
	cdataStart := []byte("<![CDATA[")
	cdataEnd := []byte("]]>")
	i := 0
	for i < len(data) {
		// Pass CDATA sections through unchanged.
		if bytes.HasPrefix(data[i:], cdataStart) {
			end := bytes.Index(data[i+len(cdataStart):], cdataEnd)
			if end >= 0 {
				end += i + len(cdataStart) + len(cdataEnd)
				buf.Write(data[i:end])
				i = end
				continue
			}
			// Unclosed CDATA — emit the rest as-is.
			buf.Write(data[i:])
			break
		}
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
