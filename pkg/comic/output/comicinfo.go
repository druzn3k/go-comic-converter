package output

import "encoding/xml"

// ComicInfo represents the ComicInfo.xml metadata inside a CBZ archive.
// It follows the ComicRack/ComicInfo schema widely used by comic readers.
type ComicInfo struct {
	XMLName   xml.Name `xml:"ComicInfo"`
	XmlnsXsd  string   `xml:"xmlns:xsd,attr"`
	XmlnsXsi  string   `xml:"xmlns:xsi,attr"`
	Title     string   `xml:"Title,omitempty"`
	Series    string   `xml:"Series,omitempty"`
	Number    string   `xml:"Number,omitempty"`
	Summary   string   `xml:"Summary,omitempty"`
	Publisher string   `xml:"Publisher,omitempty"`
	Genre     string   `xml:"Genre,omitempty"`
	PageCount int      `xml:"PageCount"`
	Writer    string   `xml:"Writer,omitempty"`
	Manga     string   `xml:"Manga,omitempty"`
	Notes     string   `xml:"Notes,omitempty"`
}

// MarshalComicInfo serializes a ComicInfo XML document from part metadata.
func MarshalComicInfo(meta PartMetadata, pageCount int) ([]byte, error) {
	ci := ComicInfo{
		XmlnsXsd:  "http://www.w3.org/2001/XMLSchema",
		XmlnsXsi:  "http://www.w3.org/2001/XMLSchema-instance",
		Title:     meta.Title,
		Series:    meta.Series,
		Number:    meta.Number,
		Summary:   meta.Summary,
		Publisher: meta.Publisher,
		Genre:     meta.Genre,
		PageCount: pageCount,
		Writer:    meta.Author,
		Manga:     meta.Manga,
	}
	data, err := xml.MarshalIndent(ci, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), data...), nil
}
