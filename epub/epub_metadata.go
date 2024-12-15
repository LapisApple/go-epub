package epub

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/lapisapple/epubreader/epub/quant"
)

// Metadata contains publishing information about the epub.
type Metadata struct {
	Title       Title     `xml:"title"`
	Language    string    `xml:"language"`
	Identifier  []string  `xml:"identifier"`
	Creator     []Creator `xml:"creator"`
	Contributor []Creator `xml:"contributor"`
	Publisher   Refinable `xml:"publisher"`
	Subject     string    `xml:"subject"`
	Description string    `xml:"description"`
	Event       []struct {
		// never seen Name used
		Name string `xml:"event,attr"`
		Date string `xml:",innerxml"`
	} `xml:"date"`
	Type     string `xml:"type"`
	Format   string `xml:"format"`
	Source   string `xml:"source"`
	Relation string `xml:"relation"`
	Coverage string `xml:"coverage"`
	Rights   string `xml:"rights"`
	// custom
	OtherTags       map[string][]string `xml:"-"`
	CoverManifestId string              `xml:"-"`
	// might contain duplicates
	PrimaryWritingMode []string `xml:"-"`
}

type MetaTags struct {
	Metatags []MetaTag `xml:"meta"`
}

type MetaTag struct {
	Name     string `xml:"name,attr"`
	Content  string `xml:"content,attr"`
	Refines  string `xml:"refines,attr"`
	Property string `xml:"property,attr"`
	InnerXML string `xml:",innerxml"`
	// not needed: scheme, ...
}

type Refinable struct {
	Name   string `xml:",innerxml"`
	ID     string `xml:"id,attr"`
	FileAs string `xml:"file-as,attr"`
}

type Creator struct {
	Refinable
	// only "aut" || "ill" || "bkp" || ...
	CreatorRole string `xml:"role,attr"`
	DisplaySeq  string `xml:"display-seq,attr"`
}

func (c *Creator) setNullFields(refineFileAs, refinesCreatorType, refinesCreatorDisplaySeq map[string]string) {
	if len(c.ID) == 0 {
		return
	}
	setNullField(refineFileAs, c.ID, &c.FileAs)
	setNullField(refinesCreatorType, c.ID, &c.CreatorRole)
	setNullField(refinesCreatorDisplaySeq, c.ID, &c.DisplaySeq)
}

func setNullField(data map[string]string, key string, res *string) {
	if newValue, ok := data[key]; ok && len(*res) == 0 {
		*res = newValue
		if debugEpubMetadata {
			delete(data, key)
		}
	}
}

func appendNonNullField(data map[string]string, key string, res *[]string) {
	if newValue, ok := data[key]; ok {
		if *res == nil {
			*res = make([]string, 0)
		}
		*res = append(*res, newValue)
		if debugEpubMetadata {
			delete(data, key)
		}
	}
}

type Title struct {
	Refinable
	TitleType string `xml:"title-type,attr"`
}

func getMetadataFromFileBytes(data []byte) ([]byte, error) {
	_, suffix, ok := bytes.Cut(data, []byte("<metadata"))
	if !ok {
		return nil, fmt.Errorf("metadata not found 1")
	}
	_, suffix, ok = bytes.Cut(suffix, []byte(">"))
	if !ok {
		return nil, fmt.Errorf("metadata not found 2")
	}
	data, _, ok = bytes.Cut(suffix, []byte("</metadata>"))
	if !ok {
		return nil, fmt.Errorf("metadata not found 3")
	}
	data = []byte("<metadata>" + string(data) + "</metadata>")

	return data, nil
}

var tempData = make(map[string][]string)

const debugEpubMetadata = false
const debugPrintOtherTags = false
const debugPrintParsedMetaTags = false

// setContainer unmarshals the epub's container.xml file.
func (rf *Rootfile) unmarshallCustomMetadata(data []byte) error {
	customMetadata := MetaTags{}

	data, err := getMetadataFromFileBytes(data)
	if err != nil {
		return err
	}
	err = xml.Unmarshal(data, &customMetadata)
	if err != nil {
		return err
	}

	if debugPrintParsedMetaTags {
		quant.PrettyPrint("customMetadata:\n %s\n", customMetadata)
	}

	refinesFileAs := make(map[string]string)
	refinesCreatorType := make(map[string]string)
	refinesDCTerms := make(map[string]string)
	refinesCreatorDisplaySeq := make(map[string]string)
	refineTitleType := make(map[string]string)

	rf.Metadata.OtherTags = make(map[string][]string)

	// parse []meta
	for _, meta := range customMetadata.Metatags {
		if len(meta.Name) > 0 && len(meta.Content) > 0 {
			if meta.Name == "cover" {
				// only set cover if it hasn't been set yet
				if len(rf.Metadata.CoverManifestId) == 0 {
					rf.Metadata.CoverManifestId = meta.Content
				}
			} else {
				if _, ok := rf.Metadata.OtherTags[meta.Name]; !ok {
					rf.Metadata.OtherTags[meta.Name] = make([]string, 0)
				}
				rf.Metadata.OtherTags[meta.Name] = append(rf.Metadata.OtherTags[meta.Name], meta.Content)
			}
		}

		if len(meta.Refines) > 0 {
			refines := meta.Refines[1:]
			switch meta.Property {
			case "file-as":
				refinesFileAs[refines] = meta.InnerXML
			case "role":
				refinesCreatorType[refines] = meta.InnerXML
			case "display-seq":
				refinesCreatorDisplaySeq[refines] = meta.InnerXML
			case "title-type":
				refineTitleType[refines] = meta.InnerXML
			// add more cases here if needed
			// i.e. like "title-type", "display-seq", ...
			default:
				// is dc term (i.e. "dcterms:title", "dcterms:language", "dcterms:identifier", "dcterms:creator", ...)
				key, isDCTerm := strings.CutPrefix(meta.Property, "dcterms:")
				if isDCTerm {
					refinesDCTerms[key] = meta.InnerXML
					continue
				}

				// debugging purposes (info about unknown meta tags)
				if debugEpubMetadata {
					if _, ok := tempData[meta.Property]; !ok {
						tempData[meta.Property] = make([]string, 0)
					}
					tempData[meta.Property] = append(tempData[meta.Property], refines+":-:"+meta.InnerXML)
				}
			}
		} else if len(meta.Property) > 0 {
			rf.Metadata.OtherTags[meta.Property] = append(rf.Metadata.OtherTags[meta.Property], meta.InnerXML)
		}
	}

	// try setting dcterms
	setNullField(refinesDCTerms, "title", &rf.Metadata.Title.Name)
	setNullField(refinesDCTerms, "language", &rf.Metadata.Language)
	// set id docterms
	appendNonNullField(refinesDCTerms, "identifier", &rf.Metadata.Identifier)
	appendNonNullField(refinesDCTerms, rf.UniqueIdentifier, &rf.Metadata.Identifier)
	// hard to implement for creator and pretty useless because ID would be known and points somewhere,
	// which likely means that the creator name is already known
	// setNullField(refinesDCTerms, "creator", &rf.Metadata.Creator[?].Name)

	// set title and file-as + title-type
	setNullField(refinesFileAs, rf.Metadata.Title.ID, &rf.Metadata.Title.FileAs)
	setNullField(refineTitleType, rf.Metadata.Title.ID, &rf.Metadata.Title.TitleType)

	// set publisher file-as
	setNullField(refinesFileAs, "publisher", &rf.Metadata.Publisher.FileAs)

	// set creator file-as + role + display-seq
	for i := range rf.Metadata.Creator {
		c := &rf.Metadata.Creator[i]
		c.setNullFields(refinesFileAs, refinesCreatorType, refinesCreatorDisplaySeq)
	}

	// set contributor file-as + role (+ display-seq)
	for i := range rf.Metadata.Contributor {
		c := &rf.Metadata.Contributor[i]
		c.setNullFields(refinesFileAs, refinesCreatorType, refinesCreatorDisplaySeq)
	}

	// get primary writing mode
	if primaryWritingMode := rf.Metadata.OtherTags["primary-writing-mode"]; len(primaryWritingMode) > 0 {
		if len(rf.Metadata.PrimaryWritingMode) == 0 {
			rf.Metadata.PrimaryWritingMode = primaryWritingMode
		} else {
			rf.Metadata.PrimaryWritingMode = append(rf.Metadata.PrimaryWritingMode, primaryWritingMode...)
		}
		delete(rf.Metadata.OtherTags, "primary-writing-mode")
	}

	// debugging purposes
	if debugEpubMetadata {
		fmt.Println("Start DebugUnusedEpubMetadata Printing...")
		// refines

		if len(refinesFileAs) > 0 {
			quant.PrettyPrint("refinesFileAs:\n %s\n", refinesFileAs)
		}
		if len(refinesCreatorType) > 0 {
			quant.PrettyPrint("refinesCreatorType:\n %s\n", refinesCreatorType)
		}
		if len(refinesDCTerms) > 0 {
			quant.PrettyPrint("refinesDCTerms:\n %s\n", refinesDCTerms)
		}
		if len(refinesCreatorDisplaySeq) > 0 {
			quant.PrettyPrint("refinesCreatorDisplaySeq:\n %s\n", refinesCreatorDisplaySeq)
		}
		if len(refineTitleType) > 0 {
			quant.PrettyPrint("refineTitleType:\n %s\n", refineTitleType)
		}
		if len(tempData) > 0 {
			quant.PrettyPrint("tempData:\n %s\n", tempData)
		}
		fmt.Println("...End DebugUnusedEpubMetadata Printing")
	}

	if debugPrintOtherTags {
		fmt.Println("Start DebugOtherTags Printing...")
		if len(rf.Metadata.OtherTags) > 0 {
			quant.PrettyPrint("OtherTags:\n %s\n", rf.Metadata.OtherTags)
		}
		fmt.Println("...End DebugOtherTags Printing")
	}

	return nil
}
