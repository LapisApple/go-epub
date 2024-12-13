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

// setContainer unmarshals the epub's container.xml file.
func (rf *Rootfile) unmarshallCustomMetadata(data []byte) error {
	customMetadata := MetaTags{}
	_, suffix, ok := bytes.Cut(data, []byte("<metadata"))
	if !ok {
		return fmt.Errorf("metadata not found 1")
	}
	_, suffix, ok = bytes.Cut(suffix, []byte(">"))
	if !ok {
		return fmt.Errorf("metadata not found 2")
	}
	data, _, ok = bytes.Cut(suffix, []byte("</metadata>"))
	if !ok {
		return fmt.Errorf("metadata not found 3")
	}
	data = []byte("<metadata>" + string(data) + "</metadata>")
	// fmt.Printf("data %s\n", data)
	err := xml.Unmarshal(data, &customMetadata)
	if err != nil {
		fmt.Printf("\033[0;31merror %v\n\033[0m", err)
		return err
	}
	//
	quant.PrettyPrint("customMetadata:\n %s\n", customMetadata)
	//
	// rf.Metadata.OtherTags = make(map[string][]string)
	// for k, v := range customMetadata.OtherTags {
	// 	if len(v) == 0 {
	// 		continue
	// 	}
	// 	rf.Metadata.OtherTags[k] = make([]string, 0)
	// 	for _, v := range v {
	// 		if v == nil || len(*v) == 0 {
	// 			continue
	// 		}
	// 		rf.Metadata.OtherTags[k] = append(rf.Metadata.OtherTags[k], *v)
	// 	}
	// }
	// if len(rf.Metadata.CoverManifestId) == 0 {
	// 	rf.Metadata.CoverManifestId = customMetadata.CoverId
	// }
	// if title := customMetadata.RefinesMap["title"]; title != nil {
	// 	rf.Metadata.Title.FileAs = *title
	// }
	// if publisher := customMetadata.RefinesMap["publisher"]; publisher != nil {
	// 	rf.Metadata.Publisher.FileAs = *publisher
	// }
	// //
	// for i := range rf.Metadata.Creator {
	// 	realCreator := &rf.Metadata.Creator[i]
	// 	if creatorFileAs := customMetadata.RefinesMap[realCreator.ID]; creatorFileAs != nil {
	// 		realCreator.FileAs = *creatorFileAs
	// 	} else {
	// 		fmt.Printf("\033[0;31mcreator %s not found\n\033[0m", realCreator.ID)
	// 	}
	// }

	return nil
}
