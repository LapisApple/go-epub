package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path"
)

const containerPath = "META-INF/container.xml"

var (
	// ErrNoRootfile occurs when there are no rootfile entries found in
	// container.xml.
	ErrNoRootfile = errors.New("epub: no rootfile found in container")

	// ErrBadRootfile occurs when container.xml references a rootfile that does
	// not exist in the zip.
	ErrBadRootfile = errors.New("epub: container references non-existent rootfile")

	// ErrNoItemref occurrs when a content.opf contains a spine without any
	// itemref entries.
	ErrNoItemref = errors.New("epub: no itemrefs found in spine")

	// ErrBadItemref occurs when an itemref entry in content.opf references an
	// item that does not exist in the manifest.
	ErrBadItemref = errors.New("epub: itemref references non-existent item")

	// ErrBadManifest occurs when a manifest in content.opf references an item
	// that does not exist in the zip.
	ErrBadManifest = errors.New("epub: manifest references non-existent item")

	// ErrMissingCoverId occurs when the epub does not have a cover image.
	ErrMissingCoverId = errors.New("epub: missing cover id in metadata")
)

// Reader represents a readable epub file.
type Reader struct {
	Container
	files map[string]*zip.File
}

// ReadCloser represents a readable epub file that can be closed.
type ReadCloser struct {
	Reader
	f      *os.File
	F_SIZE int64
}

// Rootfile contains the location of a content.opf package file.
type Rootfile struct {
	FullPath string `xml:"full-path,attr"`
	Package
	Toc Toc
}

type Toc struct {
	DocTitle  string     `xml:"docTitle>text"`
	NavPoints []NavPoint `xml:"navMap>navPoint"`
}

// Container serves as a directory of Rootfiles.
type Container struct {
	Rootfiles []*Rootfile `xml:"rootfiles>rootfile"`
}

// Package represents an epub content.opf file.
type Package struct {
	UniqueIdentifier string   `xml:"unique-identifier,attr"`
	Metadata         Metadata `xml:"metadata"`
	Manifest
	Spine Spine `xml:"spine"`
}

// Manifest lists every file that is part of the epub.
type Manifest struct {
	Items []ManifestItem `xml:"manifest>item"`
}

// ManifestItem represents a file stored in the epub.
type ManifestItem struct {
	ID        string `xml:"id,attr"`
	HREF      string `xml:"href,attr"`
	MediaType string `xml:"media-type,attr"`
	// properties are skipped by the rust crate
	Properties string `xml:"properties,attr"`
	f          *zip.File
}

// Spine defines the reading order of the epub documents.
type Spine struct {
	Itemrefs []SpineItem `xml:"itemref"`
	Toc      string      `xml:"toc,attr"`
	PPD      string      `xml:"page-progression-direction,attr"`
}

// SpineItem points to an Item.
type SpineItem struct {
	SpineItemData
	*ManifestItem `xml:"-" json:"-"`
}

type SpineItemData struct {
	IDREF           string `xml:"idref,attr" json:",omitempty"`
	Linear          string `xml:"linear,attr" json:",omitempty"`
	SpineProperties string `xml:"properties,attr" json:",omitempty"`
	// have never seen this irl, but the rust crate has it so i guess it's real
	SpineID string `xml:"id,attr" json:",omitempty"`
}

// OpenReader will open the epub file specified by name and return a
// ReadCloser.
func OpenReader(name string) (*ReadCloser, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	return NewReader(f)
}

func NewReader(f *os.File) (*ReadCloser, error) {
	rc := new(ReadCloser)
	rc.f = f

	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	fsize := fi.Size()
	rc.F_SIZE = fsize

	z, err := zip.NewReader(f, fsize)
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

func (r *Reader) init(z *zip.Reader) error {
	// Create a file lookup table
	r.files = make(map[string]*zip.File)
	for _, f := range z.File {
		r.files[f.Name] = f
	}

	err := r.setContainer()
	if err != nil {
		return err
	}
	err = r.setPackages()
	if err != nil {
		return err
	}
	err = r.setItems()
	if err != nil {
		return err
	}

	err = r.setToc()
	if err != nil {
		return err
	}

	return nil
}

// setContainer unmarshals the epub's container.xml file.
func (r *Reader) setContainer() error {
	f, err := r.files[containerPath].Open()
	if err != nil {
		return err
	}

	var b bytes.Buffer
	_, err = io.Copy(&b, f)
	if err != nil {
		return err
	}

	err = xml.Unmarshal(b.Bytes(), &r.Container)
	// todo: read more attributes out of container.xml
	if err != nil {
		return err
	}

	if len(r.Container.Rootfiles) < 1 {
		return ErrNoRootfile
	}

	return nil
}

// setPackages unmarshal's each of the epub's content.opf files.
func (r *Reader) setPackages() error {
	for _, rf := range r.Container.Rootfiles {
		if r.files[rf.FullPath] == nil {
			return ErrBadRootfile
		}

		f, err := r.files[rf.FullPath].Open()
		if err != nil {
			return err
		}

		var b bytes.Buffer
		_, err = io.Copy(&b, f)
		if err != nil {
			return err
		}

		err = xml.Unmarshal(b.Bytes(), &rf.Package)
		// todo: read more attributes out of content.opf
		if err != nil {
			return err
		}

		// set cover-image
		for _, manifestItem := range rf.Package.Manifest.Items {
			if manifestItem.Properties == "cover-image" {
				rf.Package.Metadata.CoverManifestId = manifestItem.ID
				break
			}
		}

		// parse custom metadata i.e. <meta> tags in the <metadata> tag
		err = rf.unmarshallCustomMetadata(b.Bytes())
		if err != nil {
			return err
		}
	}

	return nil
}

// setItems associates Itemrefs with their respective Item and Items with
// their zip.File.
func (r *Reader) setItems() error {
	itemrefCount := 0
	for _, rf := range r.Container.Rootfiles {
		itemMap := make(map[string]*ManifestItem)
		for i := range rf.Manifest.Items {
			item := &rf.Manifest.Items[i]
			itemMap[item.ID] = item

			abs := path.Join(path.Dir(rf.FullPath), item.HREF)
			item.f = r.files[abs]
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

type NavPoint struct {
	ID        string `xml:"id,attr"`
	PlayOrder string `xml:"playOrder,attr"`
	Label     string `xml:"navLabel>text"`
	Content   []struct {
		Path string `xml:"src,attr"`
	} `xml:"content"`
	// haven't seen this irl, but the rust crate has it so i guess it's real
	Children []NavPoint `xml:"navPoint"`
}

func (r *Reader) setToc() error {
	for _, rf := range r.Container.Rootfiles {
		tocId := rf.Spine.Toc
		if len(tocId) == 0 {
			continue
		}

		tocPath := ""
		for _, item := range rf.Manifest.Items {
			if item.ID != tocId {
				continue
			}
			tocPath = item.HREF
			break
		}

		if len(tocPath) == 0 {
			return errors.New("epub: toc item not found")
		}

		absolutPath := path.Join(path.Dir(rf.FullPath), tocPath)
		tocFile, ok := r.files[absolutPath]
		if !ok {
			return errors.New("epub: toc zip file not in epub zip map")
		}

		navPoints, err := parseTocFile(tocFile)
		if err != nil {
			return err
		}

		// set stuff
		rf.Toc = navPoints
	}

	// can't return error on (tocAmount == 0) here because some epubs don't have a toc
	return nil
}

func parseTocFile(tocFile *zip.File) (Toc, error) {
	toc := Toc{}

	f, err := tocFile.Open()
	if err != nil {
		return Toc{}, err
	}

	var b bytes.Buffer
	_, err = io.Copy(&b, f)
	if err != nil {
		return Toc{}, err
	}

	err = xml.Unmarshal(b.Bytes(), &toc)
	if err != nil {
		return Toc{}, err
	}

	return toc, nil
}

// Open returns a ReadCloser that provides access to the Items's contents.
// Multiple items may be read concurrently.
func (item *ManifestItem) Open() (r io.ReadCloser, err error) {
	if item.f == nil {
		return nil, ErrBadManifest
	}

	return item.f.Open()
}

// Close closes the epub file, rendering it unusable for I/O.
func (rc *ReadCloser) Close() {
	rc.f.Close()
}

func (r Reader) GetCover() (image *zip.File, mediaType string, err error) {
	if len(r.Container.Rootfiles) == 0 {
		return nil, "", ErrNoRootfile
	}

	hasCoverId := false
	for _, rf := range r.Container.Rootfiles {

		coverId := rf.Metadata.CoverManifestId

		if len(coverId) == 0 {
			continue
		}
		hasCoverId = true

		for _, item := range rf.Manifest.Items {
			if item.ID == coverId {
				return item.f, item.MediaType, nil
			}

		}
	}

	if hasCoverId {
		return nil, "", ErrBadManifest
	} else {
		return nil, "", ErrMissingCoverId
	}
}
