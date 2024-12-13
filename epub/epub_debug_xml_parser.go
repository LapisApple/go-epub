//go:build debug_xml_parser

// example for debug run command: "go run -tags debug_xml_parser .\main.go"
package epub

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
)

func (m *ManifestItem) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type Alias ManifestItem
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	knownFields := map[string]bool{
		"id":         true,
		"href":       true,
		"media-type": true,
		"properties": true,
	}

	for _, attr := range start.Attr {
		if !knownFields[attr.Name.Local] {
			return errors.New("epub: unknown field in manifest item: " + attr.Name.Local)
		}
	}

	return d.DecodeElement(aux, &start)
}

func (m *SpineItem) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type Alias SpineItemData
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(&m.SpineItemData),
	}

	knownFields := map[string]bool{
		"idref":      true,
		"linear":     true,
		"properties": true,
	}

	for _, attr := range start.Attr {
		if !knownFields[attr.Name.Local] {
			return errors.New("epub: unknown field in manifest item: " + attr.Name.Local)
		}
	}

	return d.DecodeElement(aux, &start)
}

func (m *NavPoint) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type Alias NavPoint
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	knownFields := map[string]bool{
		"id":        true,
		"playOrder": true,
		// useless
		"class": true,
	}

	knownChildren := map[string]bool{
		"navLabel": true,
		"content":  true,
	}

	for _, attr := range start.Attr {
		if !knownFields[attr.Name.Local] {
			return errors.New("epub: unknown field in manifest item: " + attr.Name.Local)
		}
	}

	err := d.DecodeElement(aux, &start)
	if err != nil {
		return err
	}

	// this is useless because the stream is already used up
	// due to the DecodeElement call
	for {
		t, err := d.Token()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		switch se := t.(type) {
		case xml.StartElement:
			if !knownChildren[se.Name.Local] {
				return fmt.Errorf("error unknown child: %s", se.Name.Local)
			}
		case xml.EndElement:
			if se.Name == start.Name {
				return nil
			}
		}
	}
}
