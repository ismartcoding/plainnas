package dlna

import (
	"encoding/xml"
	"fmt"
	"strings"
)

type didlLite struct {
	XMLName   xml.Name   `xml:"DIDL-Lite"`
	Xmlns     string     `xml:"xmlns,attr"`
	XmlnsDC   string     `xml:"xmlns:dc,attr"`
	XmlnsUPNP string     `xml:"xmlns:upnp,attr"`
	XmlnsDLNA string     `xml:"xmlns:dlna,attr,omitempty"`
	Items     []didlItem `xml:"item"`
}

type didlItem struct {
	ID         string  `xml:"id,attr"`
	ParentID   string  `xml:"parentID,attr"`
	Restricted int     `xml:"restricted,attr"`
	Title      string  `xml:"dc:title"`
	Class      string  `xml:"upnp:class"`
	Res        didlRes `xml:"res"`
}

type didlRes struct {
	ProtocolInfo string `xml:"protocolInfo,attr"`
	URL          string `xml:",chardata"`
}

func didlLiteMetadata(mediaURL string, title string, mime string, mediaType MediaType) (string, error) {
	upnpClass := "object.item"
	switch mediaType {
	case MediaTypeVideo:
		upnpClass = "object.item.videoItem"
	case MediaTypeAudio:
		upnpClass = "object.item.audioItem.musicTrack"
	case MediaTypeImage:
		upnpClass = "object.item.imageItem.photo"
	}

	if strings.TrimSpace(title) == "" {
		title = "PlainNAS"
	}
	if strings.TrimSpace(mime) == "" {
		mime = "application/octet-stream"
	}

	d := didlLite{
		Xmlns:     "urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/",
		XmlnsDC:   "http://purl.org/dc/elements/1.1/",
		XmlnsUPNP: "urn:schemas-upnp-org:metadata-1-0/upnp/",
		XmlnsDLNA: "urn:schemas-dlna-org:metadata-1-0/",
		Items: []didlItem{
			{
				ID:         "0",
				ParentID:   "0",
				Restricted: 1,
				Title:      title,
				Class:      upnpClass,
				Res: didlRes{
					ProtocolInfo: fmt.Sprintf("http-get:*:%s:*", mime),
					URL:          mediaURL,
				},
			},
		},
	}

	b, err := xml.Marshal(d)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
