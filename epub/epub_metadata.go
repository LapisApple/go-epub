package epub

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"github.com/lapisapple/epubreader/epub/quant"
)

// Metadata contains publishing information about the epub.
type Metadata struct {
	Title struct {
		Name   string `xml:",innerxml"`
		FileAs string `xml:"file-as,attr"`
	} `xml:"title"`
	Language    string    `xml:"language"`
	Identifier  string    `xml:"idenifier"`
	Creator     []Creator `xml:"creator"` // author
	Contributor string    `xml:"contributor"`
	Publisher   struct {
		Name   string `xml:",innerxml"`
		FileAs string `xml:"file-as,attr"`
	} `xml:"publisher"`
	Subject     string `xml:"subject"`
	Description string `xml:"description"`
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
	OtherTags       map[string][]string `xml:"-"`
	CoverManifestId string              `xml:"-"`
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

type Creator struct {
	Name   string `xml:",innerxml"`
	ID     string `xml:"id,attr"`
	FileAs string `xml:"file-as,attr"`
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

			// add more cases here if needed
			// i.e. "title-type", "role", ...
			default:
			}
		} else if len(meta.Property) > 0 {
			rf.Metadata.OtherTags[meta.Property] = append(rf.Metadata.OtherTags[meta.Property], meta.InnerXML)
		}
	}

	// set title and publisher file-as
	if title, ok := refineFileAs["title"]; ok {
		rf.Metadata.Title.FileAs = title
	}
	if publisher, ok := refineFileAs["publisher"]; ok {
		rf.Metadata.Publisher.FileAs = publisher
	}

	// set creator file-as
	for i := range rf.Metadata.Creator {
		c := &rf.Metadata.Creator[i]
		if creatorFileAs, ok := refineFileAs[c.ID]; ok {
			c.FileAs = creatorFileAs
		} else {
			quant.PrintError("creator %s not found\n", c.ID)
		}
	}

	quant.PrettyPrint("NewCreators:\n %s\n", rf.Metadata.Creator)
	quant.PrettyPrint("NewOtherTags:\n %s\n", rf.Metadata.OtherTags)

	return nil
}
