package epub

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/lapisapple/epubreader/epub/quant"
)

// for r in &manifest.borrow().children {
// 	let item = r.borrow();
// 	if self.cover_id.is_none() {
// 		if let (Some(id), Some(property)) = (item.get_attr("id"), item.get_attr("properties")) {
// 			if property == "cover-image" {
// 				self.cover_id = Some(id);
// 			}
// 		}
// 	}
// 	let _ = self.insert_resource(&item);
// }

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
	// data
	// InnerXml string `xml:",innerxml"`
	// custom
	OtherTags       map[string][]string `xml:"-"`
	CoverManifestId string              `xml:"-"`
	// TitleFileAs     string              `xml:"-"`
	// PublisherFileAs string              `xml:"-"`
	// CreatorFileAs   string    `xml:"-"`
}

type CustomMetadataInfo struct {
	OtherTags map[string][]*string `xml:"-"`
	CoverId   string               `xml:"-"`
	// TitleFileAs     string              `xml:"-"`
	// PublisherFileAs string              `xml:"-"`
	// refinesId -> FileAs
	RefinesMap map[string]*string `xml:"-"`
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
}

type Creator struct {
	Name   string `xml:",innerxml"`
	ID     string `xml:"id,attr"`
	FileAs string `xml:"file-as,attr"`
}

func (m *CustomMetadataInfo) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	m.OtherTags = make(map[string][]*string)
	m.RefinesMap = make(map[string]*string)

	// fmt.Printf("test startElement %v\n", start)

	// count := 0
	// defer fmt.Printf("\033[0;31mcount: %d\n\033[0m", count)

	var currentTextSink *string = nil

	for {
		t, err := d.Token()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		// count++

		// fmt.Printf("test tokenLocalName %s\n", t.)
		switch tt := t.(type) {

		// TODO: parse the inner structure

		case xml.StartElement:
			// fmt.Println("test > ", tt)

			switch tt.Name.Local {
			case "meta":

				importantData := map[string]string{
					"refines":  "",
					"content":  "",
					"property": "",
					"name":     "",
					// useless
					"scheme": "",
				}

				for _, attr := range tt.Attr {
					if _, ok := importantData[attr.Name.Local]; ok {
						importantData[attr.Name.Local] = attr.Value
					} else {
						return fmt.Errorf("unknown attribute %s in metadata>meta", attr.Name.Local)
					}
				}

				name := importantData["name"]
				content := importantData["content"]
				if len(content) > 0 && len(name) > 0 {
					if importantData["name"] == "cover" {
						m.CoverId = content
					} else {
						if _, ok := m.OtherTags[name]; !ok {
							m.OtherTags[name] = make([]*string, 0)
						}
						m.OtherTags[name] = append(m.OtherTags[name], &content)
					}

				}

				property := importantData["property"]
				refines := importantData["refines"]
				if len(refines) > 0 && property == "file-as" {
					refinesId := refines[1:]
					// refinesId -> internal text
					// get inner xml

					s := ""
					m.RefinesMap[refinesId] = &s
					currentTextSink = &s

				} else if len(property) > 0 {
					// property -> internal text
					s := ""
					m.OtherTags[property] = append(m.OtherTags[property], &s)
					currentTextSink = &s

				}
			default:
				k := tt.Name.Local
				v := ""
				currentTextSink = &v

				knownKeys := map[string]bool{
					"title":       true,
					"language":    true,
					"identifier":  true,
					"creator":     true,
					"contributor": true,
					"publisher":   true,
					"subject":     true,
					"description": true,
					"date":        true,
					"type":        true,
				}

				if !knownKeys[k] {
					fmt.Printf("\033[0;31munknown key %s\n\033[0m", k)
				}
			}

		case xml.EndElement:
			// fmt.Println("test <", tt)
			if tt.Name == start.Name {
				return nil
			}

		case xml.CharData:
			// fmt.Printf("test charData %s\n", tt)
			stringData := strings.TrimSpace(string(tt))
			if len(stringData) == 0 {
				continue
			}
			if currentTextSink != nil {
				*currentTextSink = string(stringData)
				currentTextSink = nil
			} else {
				fmt.Printf("\033[0;31mempty char data: %s\n\033[0m", tt)
			}
		}
	}
}

// if tt.Name.Local == "meta" {
// 	name := ""
// 	content := ""
// 	property := ""
// 	for _, attr := range tt.Attr {
// 		if attr.Name.Local == "name" {
// 			name = attr.Value
// 		} else if attr.Name.Local == "content" {
// 			content = attr.Value
// 		} else if attr.Name.Local == "property" {
// 			property = attr.Value
// 		}
// 	}

// 	if len(name) > 0 && len(content) > 0 {
// 		if name == "cover" {
// 			m.CoverId = content
// 		} else {
// 			if _, ok := m.OtherTags[name]; !ok {
// 				m.OtherTags[name] = make([]string, 0)
// 			}
// 			m.OtherTags[name] = append(m.OtherTags[name], content)
// 		}
// 	} else if len(property) > 0 {
// 		if property == "cover-image" {
// 			m.CoverId = name
// 		}
// 	}
// } else {

// }

// setContainer unmarshals the epub's container.xml file.
func (rf *Rootfile) unmarshallCustomMetadata(data []byte) error {
	customMetadata := CustomMetadataInfo{}
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
	rf.Metadata.OtherTags = make(map[string][]string)
	for k, v := range customMetadata.OtherTags {
		if len(v) == 0 {
			continue
		}
		rf.Metadata.OtherTags[k] = make([]string, 0)
		for _, v := range v {
			if v == nil || len(*v) == 0 {
				continue
			}
			rf.Metadata.OtherTags[k] = append(rf.Metadata.OtherTags[k], *v)
		}
	}
	if len(rf.Metadata.CoverManifestId) == 0 {
		rf.Metadata.CoverManifestId = customMetadata.CoverId
	}
	if title := customMetadata.RefinesMap["title"]; title != nil {
		rf.Metadata.Title.FileAs = *title
	}
	if publisher := customMetadata.RefinesMap["publisher"]; publisher != nil {
		rf.Metadata.Publisher.FileAs = *publisher
	}
	//
	for i := range rf.Metadata.Creator {
		realCreator := &rf.Metadata.Creator[i]
		if creatorFileAs := customMetadata.RefinesMap[realCreator.ID]; creatorFileAs != nil {
			realCreator.FileAs = *creatorFileAs
		} else {
			fmt.Printf("\033[0;31mcreator %s not found\n\033[0m", realCreator.ID)
		}
	}

	return nil
}
