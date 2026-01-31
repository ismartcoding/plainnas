package dlna

type Renderer struct {
	UDN          string
	Name         string
	Manufacturer string
	ModelName    string
	Location     string
}

type MediaType string

const (
	MediaTypeAudio MediaType = "AUDIO"
	MediaTypeVideo MediaType = "VIDEO"
	MediaTypeImage MediaType = "IMAGE"
)

type upnpDiscovered struct {
	RemoteAddr string
	Location   string
	USN        string
	ST         string
	Server     string
}
