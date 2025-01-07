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

func main() {
	_ = certmanagerv1.AddToScheme(scheme.Scheme)
	
	// init plugin
	registerCtx := plugin.MustInit()

	// register ingress hook
	plugin.MustRegister(ingresses.NewIngressHook())

	// register certificate syncer
	syncer, err := certificates.New(registerCtx)
	if err != nil {
		klog.Fatalf("Error creating certificate syncer: %v", err)
	}
	plugin.MustRegister(syncer)

	// register issuer syncer
	issuers_syncer, err := issuers.New(registerCtx)
	plugin.MustRegister(issuers_syncer)

	// register secrets syncer
	secrets_syncer, err := secrets.New(registerCtx)
	if err != nil {
		klog.Fatalf("Error creating secrets syncer: %v", err)
	}
	plugin.MustRegister(secrets_syncer)

	plugin.MustStart()
}
