package gopub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"path"
	"strings"
)

// Package represents an epub content.opf file.
type Package struct {
	Version          string   `xml:"version,attr"`
	UniqueIdentifier string   `xml:"unique-identifier,attr"`
	Metadata         Metadata `xml:"metadata"`
	Manifest
	Spine Spine `xml:"spine"`
	Guide Guide `xml:"guide"`
}

// Manifest lists every file that is part of the epub.
type Manifest struct {
	Items []ManifestItem `xml:"manifest>item"`
}

// Stylesheets returns manifest items with media type text/css.
func (m *Manifest) Stylesheets() []*ManifestItem {
	return m.itemsByMediaTypePrefix(MediaTypeCSS)
}

// Images returns manifest items with image/* media types.
func (m *Manifest) Images() []*ManifestItem {
	return m.itemsByMediaTypePrefix("image/")
}

// Fonts returns manifest items with font/* or common font application media types.
func (m *Manifest) Fonts() []*ManifestItem {
	var out []*ManifestItem
	for i := range m.Items {
		item := &m.Items[i]
		mt := item.MediaType
		if strings.HasPrefix(mt, "font/") ||
			mt == MediaTypeEOT ||
			strings.HasPrefix(mt, "application/font-") ||
			strings.HasPrefix(mt, "application/x-font-") {
			out = append(out, item)
		}
	}
	return out
}

func (m *Manifest) itemsByMediaTypePrefix(prefix string) []*ManifestItem {
	var out []*ManifestItem
	for i := range m.Items {
		item := &m.Items[i]
		if item.MediaType == prefix || strings.HasPrefix(item.MediaType, prefix) {
			out = append(out, item)
		}
	}
	return out
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

// IsLinear reports whether this spine item is part of the primary reading order.
// Items with linear="no" are supplementary (e.g. footnotes, back-matter).
func (s *SpineItem) IsLinear() bool {
	return s.Linear != "no"
}

// Guide lists EPUB 2.0 guide references (e.g. cover page, TOC).
type Guide struct {
	References []GuideReference `xml:"reference"`
}

// GuideReference is a typed link within the EPUB 2.0 guide element.
type GuideReference struct {
	Type  string `xml:"type,attr"`
	Title string `xml:"title,attr"`
	Href  string `xml:"href,attr"`
}

// resolveSVGCover finds the raster image embedded in an SVG cover item.
// Returns (nil, nil) when no image element is found (not an error).
func resolveSVGCover(svgItem *ManifestItem, rf *Rootfile) (*ManifestItem, error) {
	rc, err := svgItem.Open()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil, err
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
		return nil, nil
	}

	// Both SVG HREF and manifest HREFs are relative to the OPF dir.
	resolved := path.Join(path.Dir(svgItem.HREF), imgHref)
	for i := range rf.Manifest.Items {
		item := &rf.Manifest.Items[i]
		if item.HREF == resolved {
			return item, nil
		}
	}
	return nil, nil
}

// resolveXHTMLCover finds the first image referenced inside an XHTML/HTML cover document.
// Returns (nil, nil) when no image element is found.
func resolveXHTMLCover(xhtmlItem *ManifestItem, rf *Rootfile) (*ManifestItem, error) {
	rc, err := xhtmlItem.Open()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil, err
	}

	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	var imgSrc string
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		local := start.Name.Local
		if local != "img" && local != "image" {
			continue
		}
		for _, attr := range start.Attr {
			if attr.Name.Local == "src" || attr.Name.Local == "href" {
				imgSrc = attr.Value
				break
			}
		}
		if imgSrc != "" {
			break
		}
	}

	if imgSrc == "" {
		return nil, nil
	}

	opfDir := path.Dir(xhtmlItem.HREF)
	// Strip fragment identifier if present.
	if i := strings.IndexByte(imgSrc, '#'); i >= 0 {
		imgSrc = imgSrc[:i]
	}
	resolved := path.Join(opfDir, imgSrc)
	for i := range rf.Manifest.Items {
		item := &rf.Manifest.Items[i]
		if item.HREF == resolved {
			return item, nil
		}
	}
	return nil, nil
}

// GetCover returns the cover image manifest item, or an error if not found.
func (r *Reader) GetCover() (*ManifestItem, error) {
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
			return unwrapCoverItem(item, rf)
		}
	}

	if hasCoverId {
		return nil, ErrBadManifest
	}

	// EPUB 2.0 guide fallback: <reference type="cover" href="...">
	for _, rf := range r.Container.Rootfiles {
		for _, ref := range rf.Guide.References {
			if ref.Type != "cover" {
				continue
			}
			opfDir := path.Dir(rf.FullPath)
			// Guide href is relative to the epub root, not the OPF dir.
			// Resolve it against the OPF directory.
			resolved := path.Join(opfDir, ref.Href)
			// Strip fragment.
			if i := strings.IndexByte(resolved, '#'); i >= 0 {
				resolved = resolved[:i]
			}
			for i := range rf.Manifest.Items {
				item := &rf.Manifest.Items[i]
				itemAbs := path.Join(opfDir, item.HREF)
				if itemAbs == resolved {
					return unwrapCoverItem(item, rf)
				}
			}
		}
	}

	return nil, ErrMissingCoverId
}

// unwrapCoverItem resolves SVG and XHTML wrappers to find the underlying image.
func unwrapCoverItem(item *ManifestItem, rf *Rootfile) (*ManifestItem, error) {
	switch item.MediaType {
	case MediaTypeSVG:
		resolved, err := resolveSVGCover(item, rf)
		if err != nil {
			return nil, err
		}
		if resolved != nil {
			return resolved, nil
		}
		// SVG has no embedded image — return the SVG itself.
		return item, nil
	case MediaTypeXHTML, MediaTypeHTML:
		resolved, err := resolveXHTMLCover(item, rf)
		if err != nil {
			return nil, err
		}
		if resolved != nil {
			return resolved, nil
		}
		// XHTML has no embedded image — return the document itself.
		return item, nil
	default:
		return item, nil
	}
}
