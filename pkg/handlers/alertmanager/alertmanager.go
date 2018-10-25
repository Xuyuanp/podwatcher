package alertmanager

import (
	"context"
	"flag"
	"fmt"
	"net/http"

	"github.com/Xuyuanp/podwatcher/pkg/handlers"
	amclient "github.com/prometheus/alertmanager/client"
	promapi "github.com/prometheus/client_golang/api"
)

var (
	alertmanagerAddress  string
	alertmanagerUsername string
	alertmanagerPassword string
)

func init() {
	flag.StringVar(&alertmanagerAddress, "alertmanager-address", "", "alertmanager server address")
	flag.StringVar(&alertmanagerUsername, "alertmanager-username", "", "alertmanager username")
	flag.StringVar(&alertmanagerPassword, "alertmanager-password", "", "alertmanager password")
}

type handler struct {
	address  string
	username string
	password string
}

func NewHandler() handlers.Handler {
	if alertmanagerAddress == "" {
		panic(fmt.Errorf("flag `--alertmanager-address` is required"))
	}
	return &handler{
		address:  alertmanagerAddress,
		username: alertmanagerUsername,
		password: alertmanagerPassword,
	}
}

type basicAuthRoundTripper struct {
	parent   http.RoundTripper
	username string
	password string
}

func newBasicAuthRoundTripper(parent http.RoundTripper, username, password string) http.RoundTripper {
	if parent == nil {
		parent = http.DefaultTransport
	}
	return &basicAuthRoundTripper{parent: parent, username: username, password: password}
}

func (rt *basicAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.username != "" && rt.password != "" {
		req.SetBasicAuth(rt.username, rt.password)
	}
	return rt.parent.RoundTrip(req)
}

func (h *handler) Handle(event *handlers.Event) error {
	client, err := promapi.NewClient(promapi.Config{
		Address:      h.address,
		RoundTripper: newBasicAuthRoundTripper(nil, h.username, h.password),
	})
	if err != nil {
		return fmt.Errorf("new http client failed: %v", err)
	}
	return amclient.NewAlertAPI(client).Push(context.Background(), amclient.Alert{
		Labels: amclient.LabelSet{
			"namespace": amclient.LabelValue(event.Namespace),
			"name":      amclient.LabelValue(event.Name),
			"container": amclient.LabelValue(event.ContainerName),
			"log":       amclient.LabelValue(event.RawLog),
			"alertname": "podwatcher",
			"severity":  "critical",
		},
		Annotations: amclient.LabelSet{
			"summary": amclient.LabelValue(fmt.Sprintf("Container %s in pod %s/%s crashed", event.ContainerName, event.Namespace, event.Name)),
			"info":    amclient.LabelValue(event.Message),
		},
	})
}
