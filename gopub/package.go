package gopub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"path"
)

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
	ID         string `xml:"id,attr"`
	HREF       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
	F          *zip.File
}

// Open returns a ReadCloser that provides access to the item's contents.
func (item *ManifestItem) Open() (io.ReadCloser, error) {
	if item.F == nil {
		return nil, ErrBadManifest
	}
	return item.F.Open()
}

// Spine defines the reading order of the epub documents.
type Spine struct {
	Itemrefs []SpineItem `xml:"itemref"`
	Toc      string      `xml:"toc,attr"`
	PPD      string      `xml:"page-progression-direction,attr"`
}

// SpineItem points to a ManifestItem.
type SpineItem struct {
	IDREF           string `xml:"idref,attr"`
	Linear          string `xml:"linear,attr"`
	SpineProperties string `xml:"properties,attr"`
	SpineID         string `xml:"id,attr"`
	*ManifestItem   `xml:"-"`
}

func resolveSVGCover(svgItem *ManifestItem, rf *Rootfile) *ManifestItem {
	rc, err := svgItem.Open()
	if err != nil {
		return nil
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil
	}

	dec := xml.NewDecoder(bytes.NewReader(data))
	var imgHref string
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "image" {
			continue
		}
		for _, attr := range start.Attr {
			if attr.Name.Local == "href" {
				imgHref = attr.Value
				break
			}
		}
		if imgHref != "" {
			break
		}
	}

	if imgHref == "" {
		return nil
	}

	// Both SVG HREF and manifest HREFs are relative to the OPF dir.
	resolved := path.Join(path.Dir(svgItem.HREF), imgHref)
	for i := range rf.Manifest.Items {
		item := &rf.Manifest.Items[i]
		if item.HREF == resolved {
			return item
		}
	}
	return nil
}

// GetCover returns the cover image manifest item, or an error if not found.
func (r Reader) GetCover() (*ManifestItem, error) {
	if len(r.Container.Rootfiles) == 0 {
		return nil, ErrNoRootfile
	}

	hasCoverId := false
	for _, rf := range r.Container.Rootfiles {
		coverId := rf.Metadata.CoverManifestId
		if len(coverId) == 0 {
			continue
		}
		hasCoverId = true

		for i := range rf.Manifest.Items {
			item := &rf.Manifest.Items[i]
			if item.ID != coverId {
				continue
			}
			if item.MediaType == "image/svg+xml" {
				if resolved := resolveSVGCover(item, rf); resolved != nil {
					return resolved, nil
				}
			}
			return item, nil
		}
	}

	if hasCoverId {
		return nil, ErrBadManifest
	}
	return nil, ErrMissingCoverId
}
