package epub

// import (
// 	"encoding/xml"
// 	"fmt"
// 	"io"
// 	"strings"
// )

// type CustomMetadataInfo struct {
// 	OtherTags map[string][]*string `xml:"-"`
// 	CoverId   string               `xml:"-"`
// 	// TitleFileAs     string              `xml:"-"`
// 	// PublisherFileAs string              `xml:"-"`
// 	// refinesId -> FileAs
// 	RefinesMap map[string]*string `xml:"-"`
// }

// func (m *CustomMetadataInfo) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
// 	m.OtherTags = make(map[string][]*string)
// 	m.RefinesMap = make(map[string]*string)

// 	// fmt.Printf("test startElement %v\n", start)

// 	// count := 0
// 	// defer fmt.Printf("\033[0;31mcount: %d\n\033[0m", count)

// 	var currentTextSink *string = nil

// 	for {
// 		t, err := d.Token()
// 		if err == io.EOF {
// 			return nil
// 		} else if err != nil {
// 			return err
// 		}

// 		// count++

// 		// fmt.Printf("test tokenLocalName %s\n", t.)
// 		switch tt := t.(type) {

// 		// TODO: parse the inner structure

// 		case xml.StartElement:
// 			// fmt.Println("test > ", tt)

// 			switch tt.Name.Local {
// 			case "meta":

// 				importantData := map[string]string{
// 					"refines":  "",
// 					"content":  "",
// 					"property": "",
// 					"name":     "",
// 					// useless
// 					"scheme": "",
// 				}

// 				for _, attr := range tt.Attr {
// 					if _, ok := importantData[attr.Name.Local]; ok {
// 						importantData[attr.Name.Local] = attr.Value
// 					} else {
// 						return fmt.Errorf("unknown attribute %s in metadata>meta", attr.Name.Local)
// 					}
// 				}

// 				name := importantData["name"]
// 				content := importantData["content"]
// 				if len(content) > 0 && len(name) > 0 {
// 					if importantData["name"] == "cover" {
// 						m.CoverId = content
// 					} else {
// 						if _, ok := m.OtherTags[name]; !ok {
// 							m.OtherTags[name] = make([]*string, 0)
// 						}
// 						m.OtherTags[name] = append(m.OtherTags[name], &content)
// 					}

// 				}

// 				property := importantData["property"]
// 				refines := importantData["refines"]
// 				if len(refines) > 0 && property == "file-as" {
// 					refinesId := refines[1:]
// 					// refinesId -> internal text
// 					// get inner xml

// 					s := ""
// 					m.RefinesMap[refinesId] = &s
// 					currentTextSink = &s

// 				} else if len(property) > 0 {
// 					// property -> internal text
// 					s := ""
// 					m.OtherTags[property] = append(m.OtherTags[property], &s)
// 					currentTextSink = &s

// 				}
// 			default:
// 				k := tt.Name.Local
// 				v := ""
// 				currentTextSink = &v

// 				knownKeys := map[string]bool{
// 					"title":       true,
// 					"language":    true,
// 					"identifier":  true,
// 					"creator":     true,
// 					"contributor": true,
// 					"publisher":   true,
// 					"subject":     true,
// 					"description": true,
// 					"date":        true,
// 					"type":        true,
// 				}

// 				if !knownKeys[k] {
// 					fmt.Printf("\033[0;31munknown key %s\n\033[0m", k)
// 				}
// 			}

// 		case xml.EndElement:
// 			// fmt.Println("test <", tt)
// 			if tt.Name == start.Name {
// 				return nil
// 			}

// 		case xml.CharData:
// 			// fmt.Printf("test charData %s\n", tt)
// 			stringData := strings.TrimSpace(string(tt))
// 			if len(stringData) == 0 {
// 				continue
// 			}
// 			if currentTextSink != nil {
// 				*currentTextSink = string(stringData)
// 				currentTextSink = nil
// 			} else {
// 				fmt.Printf("\033[0;31mempty char data: %s\n\033[0m", tt)
// 			}
// 		}
// 	}
// }
