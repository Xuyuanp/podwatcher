package console

import (
	"github.com/Xuyuanp/podwatcher/pkg/handlers"
	"github.com/golang/glog"
)

type handler struct{}

func NewHandler() handlers.Handler {
	return (*handler)(nil)
}

func (*handler) Handle(event *handlers.Event) error {
	glog.Errorf("%+v", event)
	return nil
}
