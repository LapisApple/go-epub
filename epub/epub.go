package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
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
)

// Reader represents a readable epub file.
type Reader struct {
	Container
	files map[string]*zip.File
}

// ReadCloser represents a readable epub file that can be closed.
type ReadCloser struct {
	Reader
	f *os.File
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
	UniqueIdentifier string `xml:"unique-identifier,attr"`
	Metadata
	Manifest
	Spine Spine `xml:"spine"`
}

// for r in &manifest.borrow().children {
// 	let item = r.borrow();
// 	if self.cover_id.is_none() {
// 		if let (Some(id), Some(property)) = (item.get_attr("id"), item.get_attr("properties")) {
// 			if property == "cover-image" {
// 				self.cover_id = Some(id);
// 			}
// 		}
// 	}
// 	let _ = self.insert_resource(&item);
// }

// Metadata contains publishing information about the epub.
type Metadata struct {
	Title       string `xml:"metadata>title"`
	Language    string `xml:"metadata>language"`
	Identifier  string `xml:"metadata>idenifier"`
	Creator     string `xml:"metadata>creator"`
	Contributor string `xml:"metadata>contributor"`
	Publisher   string `xml:"metadata>publisher"`
	Subject     string `xml:"metadata>subject"`
	Description string `xml:"metadata>description"`
	Event       []struct {
		Name string `xml:"event,attr"`
		Date string `xml:",innerxml"`
	} `xml:"metadata>date"`
	Type     string `xml:"metadata>type"`
	Format   string `xml:"metadata>format"`
	Source   string `xml:"metadata>source"`
	Relation string `xml:"metadata>relation"`
	Coverage string `xml:"metadata>coverage"`
	Rights   string `xml:"metadata>rights"`
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

func (m *ManifestItem) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type Alias ManifestItem
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	knownFields := map[string]bool{
		"id":         true,
		"href":       true,
		"media-type": true,
		"properties": true,
	}

	for _, attr := range start.Attr {
		if !knownFields[attr.Name.Local] {
			return errors.New("epub: unknown field in manifest item: " + attr.Name.Local)
		}
	}

	return d.DecodeElement(aux, &start)
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
	*ManifestItem `json:"-"`
}

type SpineItemData struct {
	IDREF           string `xml:"idref,attr" json:",omitempty"`
	Linear          string `xml:"linear,attr" json:",omitempty"`
	SpineProperties string `xml:"properties,attr" json:",omitempty"`
	// have never seen this irl, but the rust crate has it so i guess it's real
	SpineID string `xml:"id,attr" json:",omitempty"`
}

func (m *SpineItem) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type Alias SpineItemData
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(&m.SpineItemData),
	}

	knownFields := map[string]bool{
		"idref":      true,
		"linear":     true,
		"properties": true,
	}

	for _, attr := range start.Attr {
		if !knownFields[attr.Name.Local] {
			return errors.New("epub: unknown field in manifest item: " + attr.Name.Local)
		}
	}

	return d.DecodeElement(aux, &start)
}

// OpenReader will open the epub file specified by name and return a
// ReadCloser.
func OpenReader(name string) (*ReadCloser, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	rc := new(ReadCloser)
	rc.f = f

	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	z, err := zip.NewReader(f, fi.Size())
	if err != nil {
		return nil, err
	}

	if err = rc.init(z); err != nil {
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

func (m *NavPoint) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type Alias NavPoint
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	knownFields := map[string]bool{
		"id":        true,
		"playOrder": true,
		// useless
		"class": true,
	}

	knownChildren := map[string]bool{
		"navLabel": true,
		"content":  true,
	}

	for _, attr := range start.Attr {
		if !knownFields[attr.Name.Local] {
			return errors.New("epub: unknown field in manifest item: " + attr.Name.Local)
		}
	}

	err := d.DecodeElement(aux, &start)
	if err != nil {
		return err
	}

	for {
		t, err := d.Token()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		switch se := t.(type) {
		case xml.StartElement:
			if !knownChildren[se.Name.Local] {
				return fmt.Errorf("Error Unknown Child: %s", se.Name.Local)
			}
		case xml.EndElement:
			if se.Name == start.Name {
				return nil
			}
		}
	}
}

func (r *Reader) setToc() error {
	tocAmount := 0
	for _, rf := range r.Container.Rootfiles {
		tocId := rf.Spine.Toc
		fmt.Printf("tocId: %s\n", tocId)
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
		fmt.Printf("absolutPath: %s\n", absolutPath)
		tocFile, ok := r.files[absolutPath]
		if !ok {
			fmt.Printf("tocPath: %s\n", tocPath)
			fmt.Printf("tokFile %v\n", tocFile)
			for k, _ := range r.files {
				fmt.Printf("key: %s\n", k)
			}
			return errors.New("epub: toc zip file not in epub zip map")
		}

		navPoints, err := r.parseTocFile(tocFile)
		if err != nil {
			return err
		}

		// set stuff
		rf.Toc = navPoints
		tocAmount++
	}

	// can't return error here because some epubs don't have a toc
	// if tocAmount < 1 {
	// 	return errors.New("epub: no toc found")
	// }

	return nil
}

func (r *Reader) parseTocFile(tocFile *zip.File) (Toc, error) {
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

	fmt.Println(" -------------  current   ---------------")

	fmt.Printf("navPoints: %v\n", toc)
	fmt.Printf("fileContent %s\n", b.String())

	fmt.Println(" -------------  next   ---------------")

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
