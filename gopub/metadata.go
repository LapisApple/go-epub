package gopub

import "strings"

// Metadata contains publishing information about the epub.
type Metadata struct {
	Title       Title        `xml:"title"`
	Language    string       `xml:"language"`
	Identifier  []Identifier `xml:"identifier"`
	Creator     []Creator    `xml:"creator"`
	Contributor []Creator    `xml:"contributor"`
	Publisher   Refinable    `xml:"publisher"`
	Subject     string       `xml:"subject"`
	Description string       `xml:"description"`
	Event       []Date       `xml:"date"`
	Type        string       `xml:"type"`
	Format      string       `xml:"format"`
	Source      string       `xml:"source"`
	Relation    string       `xml:"relation"`
	Coverage    string       `xml:"coverage"`
	Rights      string       `xml:"rights"`
	// Meta holds raw <meta> tags; consumed by processRefinements, then cleared.
	Meta []MetaTag `xml:"meta"`
	// Post-processed fields (not from XML directly).
	OtherTags       map[string][]string `xml:"-"`
	CoverManifestId string              `xml:"-"`
	// might contain duplicates
	PrimaryWritingMode []string `xml:"-"`
}

// Identifier represents a dc:identifier element with optional scheme.
type Identifier struct {
	Scheme string `xml:"scheme,attr"`
	Value  string `xml:",chardata"`
}

// Date holds an event date from dc:date.
type Date struct {
	Name string `xml:"event,attr"`
	Date string `xml:",chardata"`
}

// MetaTag represents a <meta> element inside <metadata>.
type MetaTag struct {
	Name     string `xml:"name,attr"`
	Content  string `xml:"content,attr"`
	Refines  string `xml:"refines,attr"`
	Property string `xml:"property,attr"`
	InnerXML string `xml:",chardata"`
}

// Refinable is a metadata element that can carry file-as and id attributes.
type Refinable struct {
	Name   string `xml:",chardata"`
	ID     string `xml:"id,attr"`
	FileAs string `xml:"file-as,attr"`
}

// Creator represents a dc:creator or dc:contributor element.
type Creator struct {
	Refinable
	CreatorRole string `xml:"role,attr"`
	DisplaySeq  string `xml:"display-seq,attr"`
}

// Title represents a dc:title element.
type Title struct {
	Refinable
	TitleType string `xml:"title-type,attr"`
}

// processRefinements reads the Meta slice, applies EPUB 3.0 refinements and
// EPUB 2.0 name/content pairs, then clears Meta.
func processRefinements(metadata *Metadata) {
	refinesFileAs := make(map[string]string)
	refinesRole := make(map[string]string)
	refinesDisplaySeq := make(map[string]string)
	refinesTitleType := make(map[string]string)
	refinesDCTerms := make(map[string]string)

	metadata.OtherTags = make(map[string][]string)

	for _, meta := range metadata.Meta {
		if meta.Name != "" && meta.Content != "" {
			// EPUB 2.0: <meta name="..." content="...">
			if meta.Name == "cover" {
				if metadata.CoverManifestId == "" {
					metadata.CoverManifestId = meta.Content
				}
			} else {
				metadata.OtherTags[meta.Name] = append(metadata.OtherTags[meta.Name], meta.Content)
			}
			continue
		}

		if meta.Refines != "" {
			// EPUB 3.0: <meta refines="#id" property="...">value</meta>
			id := strings.TrimPrefix(meta.Refines, "#")
			switch meta.Property {
			case "file-as":
				refinesFileAs[id] = meta.InnerXML
			case "role":
				refinesRole[id] = meta.InnerXML
			case "display-seq":
				refinesDisplaySeq[id] = meta.InnerXML
			case "title-type":
				refinesTitleType[id] = meta.InnerXML
			default:
				if key, ok := strings.CutPrefix(meta.Property, "dcterms:"); ok {
					refinesDCTerms[key] = meta.InnerXML
				}
			}
			continue
		}

		if meta.Property != "" {
			// EPUB 3.0: <meta property="...">value</meta>
			metadata.OtherTags[meta.Property] = append(metadata.OtherTags[meta.Property], meta.InnerXML)
		}
	}

	// Apply dcterms overrides for standard fields.
	setIfEmpty(refinesDCTerms, "title", &metadata.Title.Name)
	setIfEmpty(refinesDCTerms, "language", &metadata.Language)
	if v, ok := refinesDCTerms["identifier"]; ok {
		metadata.Identifier = append(metadata.Identifier, Identifier{Value: v})
	}

	// Apply title refinements.
	if metadata.Title.ID != "" {
		setIfEmpty(refinesFileAs, metadata.Title.ID, &metadata.Title.FileAs)
		setIfEmpty(refinesTitleType, metadata.Title.ID, &metadata.Title.TitleType)
	}

	// Apply publisher refinements.
	if metadata.Publisher.ID != "" {
		setIfEmpty(refinesFileAs, metadata.Publisher.ID, &metadata.Publisher.FileAs)
	}

	// Apply creator/contributor refinements.
	for i := range metadata.Creator {
		applyCreatorRefinements(&metadata.Creator[i], refinesFileAs, refinesRole, refinesDisplaySeq)
	}
	for i := range metadata.Contributor {
		applyCreatorRefinements(&metadata.Contributor[i], refinesFileAs, refinesRole, refinesDisplaySeq)
	}

	// Extract primary-writing-mode into dedicated field.
	if v, ok := metadata.OtherTags["primary-writing-mode"]; ok {
		metadata.PrimaryWritingMode = append(metadata.PrimaryWritingMode, v...)
		delete(metadata.OtherTags, "primary-writing-mode")
	}

	metadata.Meta = nil
}

func applyCreatorRefinements(c *Creator, fileAs, role, displaySeq map[string]string) {
	if c.ID == "" {
		return
	}
	setIfEmpty(fileAs, c.ID, &c.FileAs)
	setIfEmpty(role, c.ID, &c.CreatorRole)
	setIfEmpty(displaySeq, c.ID, &c.DisplaySeq)
}

func setIfEmpty(m map[string]string, key string, target *string) {
	if *target == "" {
		if v, ok := m[key]; ok {
			*target = v
		}
	}
}
