package gopub

import "io"

// NCX represents an EPUB 2.0 compatible navigation document.
type NCX struct {
	DocTitle  string     `xml:"docTitle>text"`
	NavPoints []NavPoint `xml:"navMap>navPoint"`
}

// NavPoint represents a location within the epub that can be navigated to.
type NavPoint struct {
	ID        string `xml:"id,attr"`
	PlayOrder string `xml:"playOrder,attr"`
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
		tocID := rf.Spine.Toc
		if tocID == "" {
			// Fall back to the conventional "ncx" manifest ID.
			tocID = "ncx"
		}

		for _, item := range rf.Manifest.Items {
			if item.ID != tocID {
				continue
			}

			f, err := item.Open()
			if err != nil {
				return err
			}
			data, err := io.ReadAll(f)
			f.Close()
			if err != nil {
				return err
			}

			if err := xmlDecodeBytes(data, &rf.NCX); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

// ncxItemName searches the NCX for a display name matching href.
func (rf Rootfile) ncxItemName(href string) string {
	for _, point := range rf.NCX.NavPoints {
		if label := point.lookupNavPoint(href); label != "" {
			return label
		}
	}
	return ""
}

func (np NavPoint) lookupNavPoint(href string) string {
	if np.Content.Src == href {
		return np.NavLabel.Text
	}
	for _, child := range np.NavPoints {
		if label := child.lookupNavPoint(href); label != "" {
			return label
		}
	}
	return ""
}
