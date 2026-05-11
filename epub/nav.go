package epub

import (
	"io"
	"strings"
)

// NavDoc represents an EPUB 3.0 compatible navigation document.
type NavDoc struct {
	Navs []NavSection `xml:"body>nav"`
}

// NavSection represents a single <nav> element (e.g. toc, landmarks).
type NavSection struct {
	Items []NavItem `xml:"ol>li"`
}

// NavItem represents a navigable location within the epub.
type NavItem struct {
	Link struct {
		Href string `xml:"href,attr"`
		Text string `xml:",chardata"`
	} `xml:"a"`
	SubItems []NavItem `xml:"ol>li"`
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
