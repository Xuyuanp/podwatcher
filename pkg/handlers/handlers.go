package handlers

type Event struct {
	Namespace     string
	Name          string
	ContainerName string
	Reason        string
	Message       string
	RawLogs       []string
}

type Handler interface {
	Handle(*Event) error
}
