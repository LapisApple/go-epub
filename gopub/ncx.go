package gopub

import "strings"


// NCX represents an EPUB 2.0 compatible navigation document.
type NCX struct {
	DocTitle  string     `xml:"docTitle>text"`
	NavPoints []NavPoint `xml:"navMap>navPoint"`
}

// NavPoint represents a location within the epub that can be navigated to.
type NavPoint struct {
	ID        string `xml:"id,attr"`
	PlayOrder int `xml:"playOrder,attr"`
	NavLabel  struct {
		Text string `xml:"text"`
	} `xml:"navLabel"`
	Content struct {
		Src string `xml:"src,attr"`
	} `xml:"content"`
	NavPoints []NavPoint `xml:"navPoint"`
}

// setNCX loads the EPUB 2.0 NCX navigation document for each rootfile.
// Non-fatal: missing NCX is silently skipped.
func (r *Reader) setNCX() error {
	for _, rf := range r.Container.Rootfiles {
		item := r.findNCXItem(rf)
		if item == nil {
			continue
		}

		data, err := r.readItem(item)
		if err != nil {
			return err
		}

		if err := xmlDecodeBytes(data, &rf.NCX); err != nil {
			return err
		}
	}
	return nil
}

// findNCXItem locates the NCX manifest item for a rootfile.
// Prefers the spine toc attribute, then a manifest item with the
// application/x-dtbncx+xml media type, then the conventional "ncx" ID.
func (r *Reader) findNCXItem(rf *Rootfile) *ManifestItem {
	tocID := rf.Spine.Toc

	// First pass: match by spine toc ID or conventional "ncx" ID.
	// Second pass (if no toc attr): match by media type.
	var byMediaType *ManifestItem
	for i := range rf.Manifest.Items {
		item := &rf.Manifest.Items[i]
		if tocID != "" && item.ID == tocID {
			return item
		}
		if item.MediaType == MediaTypeNCX {
			if byMediaType == nil {
				byMediaType = item
			}
		}
	}
	if byMediaType != nil {
		return byMediaType
	}
	// Last resort: conventional ID "ncx".
	for i := range rf.Manifest.Items {
		item := &rf.Manifest.Items[i]
		if item.ID == "ncx" {
			return item
		}
	}
	return nil
}

// ncxItemName searches the NCX for a display name matching href.
func (rf *Rootfile) ncxItemName(href string) string {
	for _, point := range rf.NCX.NavPoints {
		if label := point.lookupNavPoint(href); label != "" {
			return label
		}
	}
	return ""
}

func (np NavPoint) lookupNavPoint(href string) string {
	src := np.Content.Src
	if i := strings.IndexByte(src, '#'); i >= 0 {
		src = src[:i]
	}
	if src == href {
		return np.NavLabel.Text
	}
	for _, child := range np.NavPoints {
		if label := child.lookupNavPoint(href); label != "" {
			return label
		}
	}
	return ""
}
