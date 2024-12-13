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
	Identifier  string    `xml:"idenifier"`
	Creator     []Creator `xml:"creator"`
	Contributor []Creator `xml:"contributor"`
	Publisher   Refinable `xml:"publisher"`
	Subject     string    `xml:"subject"`
	Description string    `xml:"description"`
	Event       []struct {
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
	OtherTags          map[string][]string `xml:"-"`
	CoverManifestId    string              `xml:"-"`
	PrimaryWritingMode string              `xml:"-"`
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

// setContainer unmarshals the epub's container.xml file.
func (rf *Rootfile) unmarshallCustomMetadata(data []byte) error {
	customMetadata := MetaTags{}

	data, err := getMetadataFromFileBytes(data)
	if err != nil {
		return err
	}
	err = xml.Unmarshal(data, &customMetadata)
	if err != nil {
		quant.PrintError("error %v\n", err)
		return err
	}
	//
	quant.PrettyPrint("customMetadata:\n %s\n", customMetadata)

	refineFileAs := make(map[string]string)
	refinesCreatorType := make(map[string]string)
	refinesDCTerms := make(map[string]string)
	refinesCreatorDisplaySeq := make(map[string]string)
	refineTitleType := make(map[string]string)

	rf.Metadata.OtherTags = make(map[string][]string)

	// parse []meta
	for _, meta := range customMetadata.Metatags {
		if len(meta.Name) > 0 && len(meta.Content) > 0 {
			if meta.Name == "cover" {
				if len(rf.Metadata.CoverManifestId) == 0 {
					rf.Metadata.CoverManifestId = meta.Content
				} else {
					quant.PrintError("cover already exists %s\n", meta.Content)
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
				refineFileAs[refines] = meta.InnerXML
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

				// is none
				if _, ok := tempData[meta.Property]; !ok {
					tempData[meta.Property] = make([]string, 0)
				}
				tempData[meta.Property] = append(tempData[meta.Property], refines+":-:"+meta.InnerXML)
			}
		} else if len(meta.Property) > 0 {
			rf.Metadata.OtherTags[meta.Property] = append(rf.Metadata.OtherTags[meta.Property], meta.InnerXML)
		}
	}

	// try setting dcterms
	if title, ok := refinesDCTerms["title"]; ok && len(rf.Metadata.Title.Name) == 0 {
		rf.Metadata.Title.Name = title
	}
	if language, ok := refinesDCTerms["language"]; ok && len(rf.Metadata.Language) == 0 {
		rf.Metadata.Language = language
	}
	if identifier, ok := refinesDCTerms["identifier"]; ok && len(rf.Metadata.Identifier) == 0 {
		rf.Metadata.Identifier = identifier
	}
	// if date, ok := refinesDCTerms["date"]; ok {
	// }
	// if creator, ok := refinesDCTerms["creator"]; ok {
	// 	rf.Metadata.Creator = append(rf.Metadata.Creator, Creator{Name: creator})
	// }

	// set title and publisher file-as
	if title, ok := refineFileAs["title"]; ok {
		rf.Metadata.Title.FileAs = title
	}
	if titleType, ok := refineTitleType[rf.Metadata.Title.ID]; ok {
		// fmt.Printf("titleTypeSet %s\n", titleType)
		rf.Metadata.Title.TitleType = titleType
	}
	if publisher, ok := refineFileAs["publisher"]; ok {
		rf.Metadata.Publisher.FileAs = publisher
	}

	// set creator file-as + role + display-seq
	for i := range rf.Metadata.Creator {
		c := &rf.Metadata.Creator[i]
		if len(c.ID) == 0 {
			continue
		}
		if creatorFileAs, ok := refineFileAs[c.ID]; ok {
			c.FileAs = creatorFileAs
		}
		if creatorRole, ok := refinesCreatorType[c.ID]; ok {
			c.CreatorRole = creatorRole
		}
		if creatorDisplaySeq, ok := refinesCreatorDisplaySeq[c.ID]; ok {
			c.DisplaySeq = creatorDisplaySeq
		}
	}

	// set contributor file-as + role
	for i := range rf.Metadata.Contributor {
		c := &rf.Metadata.Contributor[i]
		if len(c.ID) == 0 {
			continue
		}
		if contributorFileAs, ok := refineFileAs[c.ID]; ok {
			c.FileAs = contributorFileAs
		}
		if contributorRole, ok := refinesCreatorType[c.ID]; ok {
			c.CreatorRole = contributorRole
		}
	}

	// get primary writing mode
	if primaryWritingMode, ok := rf.Metadata.OtherTags["primary-writing-mode"]; ok && len(primaryWritingMode) > 0 {
		rf.Metadata.PrimaryWritingMode = primaryWritingMode[0]
		if len(primaryWritingMode) > 1 {
			quant.PrintError("primary-writing-mode more than one %s\n", primaryWritingMode)
		}
		delete(rf.Metadata.OtherTags, "primary-writing-mode")
	}

	// quant.PrettyPrint("NewCreators:\n %s\n", rf.Metadata.Creator)
	// quant.PrettyPrint("NewContributors:\n %s\n", rf.Metadata.Contributor)
	// quant.PrettyPrint("NewOtherTags:\n %s\n", rf.Metadata.OtherTags)

	quant.PrettyPrint("tempData:\n %s\n", tempData)

	return nil
}
