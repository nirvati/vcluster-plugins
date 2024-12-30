package issuers

import (
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/loft-sh/vcluster/pkg/util/translate"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *issuerSyncer) translate(vObj client.Object) *certmanagerv1.Issuer {
	vIssuer := vObj.(*certmanagerv1.Issuer)
	pObj := translate.HostMetadata(vIssuer, types.NamespacedName{Name: vObj.GetName(), Namespace: vObj.GetNamespace()})
	pObj.Spec = *rewriteSpec(&vIssuer.Spec, vIssuer.Namespace)
	return pObj
}

func (s *issuerSyncer) translateUpdate(pObj, vObj *certmanagerv1.Issuer) {
	// check annotations & labels
	pObj.Labels = translate.HostLabels(vObj, pObj)
	pObj.Annotations = translate.HostAnnotations(vObj, pObj)

	// update secret name if necessary
	pObj.Spec = *rewriteSpec(&vObj.Spec, vObj.GetNamespace()).DeepCopy()
}

func rewriteSpec(vObjSpec *certmanagerv1.IssuerSpec, namespace string) *certmanagerv1.IssuerSpec {
	// translate secret names
	vObjSpec = vObjSpec.DeepCopy()
	if vObjSpec.ACME != nil {
		vObjSpec.ACME.PrivateKey.Name = translate.Default.HostName(nil, vObjSpec.ACME.PrivateKey.Name, namespace).Name
		if vObjSpec.ACME.ExternalAccountBinding != nil {
			vObjSpec.ACME.ExternalAccountBinding.Key.Name = translate.Default.HostName(nil, vObjSpec.ACME.ExternalAccountBinding.Key.Name, namespace).Name
		}
		for i := range vObjSpec.ACME.Solvers {
			if vObjSpec.ACME.Solvers[i].DNS01 != nil {
				if vObjSpec.ACME.Solvers[i].DNS01.Akamai != nil {
					vObjSpec.ACME.Solvers[i].DNS01.Akamai.ClientToken.Name = translate.Default.HostName(nil, vObjSpec.ACME.Solvers[i].DNS01.Akamai.ClientToken.Name, namespace).Name
					vObjSpec.ACME.Solvers[i].DNS01.Akamai.ClientSecret.Name = translate.Default.HostName(nil, vObjSpec.ACME.Solvers[i].DNS01.Akamai.ClientSecret.Name, namespace).Name
					vObjSpec.ACME.Solvers[i].DNS01.Akamai.AccessToken.Name = translate.Default.HostName(nil, vObjSpec.ACME.Solvers[i].DNS01.Akamai.AccessToken.Name, namespace).Name
				}
				if vObjSpec.ACME.Solvers[i].DNS01.Cloudflare != nil {
					if vObjSpec.ACME.Solvers[i].DNS01.Cloudflare.APIKey != nil {
						vObjSpec.ACME.Solvers[i].DNS01.Cloudflare.APIKey.Name = translate.Default.HostName(nil, vObjSpec.ACME.Solvers[i].DNS01.Cloudflare.APIKey.Name, namespace).Name
					}
					if vObjSpec.ACME.Solvers[i].DNS01.Cloudflare.APIToken != nil {
						vObjSpec.ACME.Solvers[i].DNS01.Cloudflare.APIToken.Name = translate.Default.HostName(nil, vObjSpec.ACME.Solvers[i].DNS01.Cloudflare.APIToken.Name, namespace).Name
					}
				}
				if vObjSpec.ACME.Solvers[i].DNS01.DigitalOcean != nil {
					vObjSpec.ACME.Solvers[i].DNS01.DigitalOcean.Token.Name = translate.Default.HostName(nil, vObjSpec.ACME.Solvers[i].DNS01.DigitalOcean.Token.Name, namespace).Name
				}
				if vObjSpec.ACME.Solvers[i].DNS01.Route53 != nil {
					vObjSpec.ACME.Solvers[i].DNS01.Route53.SecretAccessKey.Name = translate.Default.HostName(nil, vObjSpec.ACME.Solvers[i].DNS01.Route53.SecretAccessKey.Name, namespace).Name
					if vObjSpec.ACME.Solvers[i].DNS01.Route53.SecretAccessKeyID != nil {
						vObjSpec.ACME.Solvers[i].DNS01.Route53.SecretAccessKeyID.Name = translate.Default.HostName(nil, vObjSpec.ACME.Solvers[i].DNS01.Route53.SecretAccessKeyID.Name, namespace).Name
					}
				}
				if vObjSpec.ACME.Solvers[i].DNS01.AzureDNS != nil && vObjSpec.ACME.Solvers[i].DNS01.AzureDNS.ClientSecret != nil {
					vObjSpec.ACME.Solvers[i].DNS01.AzureDNS.ClientSecret.Name = translate.Default.HostName(nil, vObjSpec.ACME.Solvers[i].DNS01.AzureDNS.ClientSecret.Name, namespace).Name
				}
				if vObjSpec.ACME.Solvers[i].DNS01.AcmeDNS != nil {
					vObjSpec.ACME.Solvers[i].DNS01.AcmeDNS.AccountSecret.Name = translate.Default.HostName(nil, vObjSpec.ACME.Solvers[i].DNS01.AcmeDNS.AccountSecret.Name, namespace).Name
				}
				if vObjSpec.ACME.Solvers[i].DNS01.RFC2136 != nil {
					vObjSpec.ACME.Solvers[i].DNS01.RFC2136.TSIGSecret.Name = translate.Default.HostName(nil, vObjSpec.ACME.Solvers[i].DNS01.RFC2136.TSIGSecret.Name, namespace).Name
				}
			}
		}
	}
	if vObjSpec.CA != nil {
		vObjSpec.CA.SecretName = translate.Default.HostName(nil, vObjSpec.CA.SecretName, namespace).Name
	}
	if vObjSpec.Vault != nil {
		if vObjSpec.Vault.Auth.TokenSecretRef != nil {
			vObjSpec.Vault.Auth.TokenSecretRef.Name = translate.Default.HostName(nil, vObjSpec.Vault.Auth.TokenSecretRef.Name, namespace).Name
		}
		if vObjSpec.Vault.CABundleSecretRef != nil {
			vObjSpec.Vault.CABundleSecretRef.Name = translate.Default.HostName(nil, vObjSpec.Vault.CABundleSecretRef.Name, namespace).Name
		}
		if vObjSpec.Vault.ClientCertSecretRef != nil {
			vObjSpec.Vault.ClientCertSecretRef.Name = translate.Default.HostName(nil, vObjSpec.Vault.ClientCertSecretRef.Name, namespace).Name
		}
		if vObjSpec.Vault.ClientKeySecretRef != nil {
			vObjSpec.Vault.ClientKeySecretRef.Name = translate.Default.HostName(nil, vObjSpec.Vault.ClientKeySecretRef.Name, namespace).Name
		}
		if vObjSpec.Vault.Auth.AppRole != nil {
			vObjSpec.Vault.Auth.AppRole.SecretRef.Name = translate.Default.HostName(nil, vObjSpec.Vault.Auth.AppRole.SecretRef.Name, namespace).Name
		}
		if vObjSpec.Vault.Auth.Kubernetes != nil {
			vObjSpec.Vault.Auth.Kubernetes.SecretRef.Name = translate.Default.HostName(nil, vObjSpec.Vault.Auth.Kubernetes.SecretRef.Name, namespace).Name
		}

	}
	if vObjSpec.Venafi != nil {
		if vObjSpec.Venafi.TPP != nil {
			vObjSpec.Venafi.TPP.CredentialsRef.Name = translate.Default.HostName(nil, vObjSpec.Venafi.TPP.CredentialsRef.Name, namespace).Name
			if vObjSpec.Venafi.TPP.CABundleSecretRef != nil {
				vObjSpec.Venafi.TPP.CABundleSecretRef.Name = translate.Default.HostName(nil, vObjSpec.Venafi.TPP.CABundleSecretRef.Name, namespace).Name
			}
		}
		if vObjSpec.Venafi.Cloud != nil {
			vObjSpec.Venafi.Cloud.APITokenSecretRef.Name = translate.Default.HostName(nil, vObjSpec.Venafi.Cloud.APITokenSecretRef.Name, namespace).Name
		}
	}
	return vObjSpec
}
