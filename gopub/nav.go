package gopub

import (
	"encoding/xml"
	"io"
	"strings"
)

// NavDoc represents an EPUB 3.0 compatible navigation document.
type NavDoc struct {
	Navs []NavSection `xml:"body>nav"`
}

// NavSection represents a single <nav> element (e.g. toc, landmarks).
// Type corresponds to the epub:type attribute (e.g. "toc", "landmarks", "page-list").
// Go's xml package matches the local name "type" regardless of namespace prefix.
type NavSection struct {
	Type  string    `xml:"type,attr"`
	Items []NavItem `xml:"ol>li"`
}

// NavItem represents a navigable location within the epub.
type NavItem struct {
	Link     navLink   `xml:"a"`
	SubItems []NavItem `xml:"ol>li"`
}

// navLink is an intermediate type for UnmarshalXML to capture all nested text.
type navLink struct {
	Href string
	Text string
}

func (l *navLink) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "href" {
			l.Href = attr.Value
		}
	}
	var sb strings.Builder
	var collectText func() error
	collectText = func() error {
		for {
			tok, err := d.Token()
			if err != nil {
				return err
			}
			switch t := tok.(type) {
			case xml.CharData:
				sb.Write(t)
			case xml.StartElement:
				if err := collectText(); err != nil {
					return err
				}
			case xml.EndElement:
				return nil
			}
		}
	}
	if err := collectText(); err != nil && err != io.EOF {
		return err
	}
	l.Text = strings.TrimSpace(sb.String())
	return nil
}

// setTOC loads the EPUB 3.0 navigation document for each rootfile.
// Non-fatal: missing nav document is silently skipped.
func (r *Reader) setTOC() error {
	for _, rf := range r.Container.Rootfiles {
		for _, item := range rf.Manifest.Items {
			if !hasNavProperty(item.Properties) {
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

			if err := xmlDecodeBytes(data, &rf.NavDoc); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

// TOCNav returns the NavSection with epub:type "toc", or nil if not found.
func (rf *Rootfile) TOCNav() *NavSection {
	for i := range rf.NavDoc.Navs {
		if rf.NavDoc.Navs[i].Type == "toc" {
			return &rf.NavDoc.Navs[i]
		}
	}
	return nil
}

// navItemName searches the NavDoc for a display name matching href.
func (rf Rootfile) navItemName(href string) string {
	for _, nav := range rf.NavDoc.Navs {
		for _, item := range nav.Items {
			if label := item.lookupNavItem(href); label != "" {
				return label
			}
		}
	}
	return ""
}

func (item NavItem) lookupNavItem(href string) string {
	if item.Link.Href == href {
		return item.Link.Text
	}
	for _, sub := range item.SubItems {
		if label := sub.lookupNavItem(href); label != "" {
			return label
		}
	}
	return ""
}

// hasNavProperty reports whether "nav" appears in a space-separated properties value.
func hasNavProperty(properties string) bool {
	for p := range strings.FieldsSeq(properties) {
		if p == "nav" {
			return true
		}
	}
	return false
}
