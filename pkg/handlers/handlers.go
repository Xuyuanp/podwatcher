package handlers

type Event struct {
	Namespace     string
	Name          string
	ContainerName string
	Reason        string
	Message       string
	RawLog        string
}

type Handler interface {
	Handle(*Event) error
}
