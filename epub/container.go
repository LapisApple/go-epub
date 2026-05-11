package epub

const containerPath = "META-INF/container.xml"

// Rootfile contains the location of an epub .opf file.
type Rootfile struct {
	FullPath string `xml:"full-path,attr"`
	Package
	NCX
	NavDoc
}

// Container serves as a directory of Rootfiles.
type Container struct {
	Rootfiles []*Rootfile `xml:"rootfiles>rootfile"`
}

// DefaultRendition returns the first rootfile, or nil if none exist.
func (c *Container) DefaultRendition() *Rootfile {
	if len(c.Rootfiles) == 0 {
		return nil
	}
	return c.Rootfiles[0]
}

// ItemName looks up a display name for the given item href.
// Tries EPUB 3.0 NavDoc first, falls back to EPUB 2.0 NCX.
func (rf *Rootfile) ItemName(href string) string {
	if label := rf.navItemName(href); label != "" {
		return label
	}
	return rf.ncxItemName(href)
}
