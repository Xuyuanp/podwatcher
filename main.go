package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/Xuyuanp/podwatcher/pkg/controller"
	"github.com/Xuyuanp/podwatcher/pkg/handlers/alertmanager"
	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfigPath string
	masterURL      string
	namespace      string
)

func init() {
	flag.StringVar(&kubeconfigPath, "kubeconfig", "", "kubeconfig")
	flag.StringVar(&masterURL, "master", "", "master URL")
	flag.StringVar(&namespace, "namespace", "", "namespace")
	flag.Parse()
}

func main() {
	config, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
	if err != nil {
		glog.Fatalf("Failed to build config from flags: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create clientset: %v", err)
	}

	versionInfo, err := clientset.Discovery().ServerVersion()
	if err != nil {
		glog.Fatalf("Failed to discover server version: %v", err)
	}
	glog.Infof("Server version: %+v", versionInfo)

	stopCh := make(chan struct{})
	sigsCh := make(chan os.Signal, 1)
	signal.Notify(sigsCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigsCh
		glog.Infof("got signal: %s", sig)
		close(stopCh)
	}()

	controller.NewController(clientset, alertmanager.NewHandler(), namespace).Run(stopCh)
}
