package main

import (
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/loft-sh/vcluster/pkg/scheme"
	"github.com/nirvati/vcluster-cert-manager-plugin/pkg/hooks/ingresses"
	"github.com/nirvati/vcluster-cert-manager-plugin/pkg/syncers/certificates"
	"github.com/nirvati/vcluster-cert-manager-plugin/pkg/syncers/issuers"
	"github.com/nirvati/vcluster-cert-manager-plugin/pkg/syncers/secrets"
	"github.com/nirvati/vcluster-sdk/plugin"
	"k8s.io/klog"
)

func init() {
	// Add cert manager types to our plugin scheme
	_ = certmanagerv1.AddToScheme(scheme.Scheme)
}

func main() {
	// init plugin
	registerCtx, err := plugin.Init()
	if err != nil {
		klog.Fatalf("Error initializing plugin: %v", err)
	}

	// register ingress hook
	err = plugin.Register(ingresses.NewIngressHook())
	if err != nil {
		klog.Fatalf("Error registering ingress hook: %v", err)
	}

	// register certificate syncer
	syncer, err := certificates.New(registerCtx)
	if err != nil {
		klog.Fatalf("Error creating certificate syncer: %v", err)
	}
	err = plugin.Register(syncer)
	if err != nil {
		klog.Fatalf("Error registering certificate syncer: %v", err)
	}

	// register issuer syncer
	issuers_syncer, err := issuers.New(registerCtx)
	err = plugin.Register(issuers_syncer)
	if err != nil {
		klog.Fatalf("Error registering certificate syncer: %v", err)
	}

	// register secrets syncer
	secrets_syncer, err := secrets.New(registerCtx)
	if err != nil {
		klog.Fatalf("Error creating secrets syncer: %v", err)
	}
	err = plugin.Register(secrets_syncer)
	if err != nil {
		klog.Fatalf("Error registering secrets syncer: %v", err)
	}

	// start plugin
	err = plugin.Start()
	if err != nil {
		klog.Fatalf("Error starting plugin: %v", err)
	}
}
