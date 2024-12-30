package certificates

import (
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/loft-sh/vcluster/pkg/mappings"
	"github.com/loft-sh/vcluster/pkg/syncer/synccontext"
	"github.com/loft-sh/vcluster/pkg/util/translate"
	"github.com/nirvati/vcluster-cert-manager-plugin/pkg/constants"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *certificateSyncer) translate(ctx *synccontext.SyncContext, vObj client.Object) *certmanagerv1.Certificate {
	pObj := translate.HostMetadata(vObj, s.VirtualToHost(ctx, types.NamespacedName{Name: vObj.GetName(), Namespace: vObj.GetNamespace()}, vObj)).(*certmanagerv1.Certificate)
	vCertificate := vObj.(*certmanagerv1.Certificate)
	rewriteSpec(ctx, &vCertificate.Spec, vCertificate.Namespace)
	return pObj
}

func (s *certificateSyncer) translateUpdate(ctx *synccontext.SyncContext, evt *synccontext.SyncEvent[*certmanagerv1.Certificate]) {
	// sync metadata
	evt.Host.Annotations = translate.HostAnnotations(evt.Virtual, evt.Host)
	evt.Host.Labels = translate.HostLabels(evt.Virtual, evt.Host)

	// sync virtual to host
	evt.Host.Spec = evt.Virtual.Spec

	// update spec
	rewriteSpec(ctx, &evt.Virtual.Spec, evt.Virtual.GetNamespace())
}

func rewriteSpec(ctx *synccontext.SyncContext, vObjSpec *certmanagerv1.CertificateSpec, namespace string) {
	if vObjSpec.SecretName != "" {
		vObjSpec.SecretName = translate.Default.HostName(ctx, vObjSpec.SecretName, namespace).Name
	}
	if vObjSpec.IssuerRef.Kind == "Issuer" {
		vObjSpec.IssuerRef.Name = translate.Default.HostName(ctx, vObjSpec.IssuerRef.Name, namespace).Name
	} else if vObjSpec.IssuerRef.Kind == "ClusterIssuer" {
		// TODO: rewrite ClusterIssuers
	}
	if vObjSpec.Keystores != nil && vObjSpec.Keystores.JKS != nil {
		vObjSpec.Keystores.JKS.PasswordSecretRef.Name = translate.Default.HostName(ctx, vObjSpec.Keystores.JKS.PasswordSecretRef.Name, namespace).Name
	}
	if vObjSpec.Keystores != nil && vObjSpec.Keystores.PKCS12 != nil {
		vObjSpec.Keystores.PKCS12.PasswordSecretRef.Name = translate.Default.HostName(ctx, vObjSpec.Keystores.PKCS12.PasswordSecretRef.Name, namespace).Name
	}
}

func (s *certificateSyncer) translateBackwards(ctx *synccontext.SyncContext, pObj *certmanagerv1.Certificate, name types.NamespacedName) (*certmanagerv1.Certificate, error) {
	vCertificate := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name.Name,
			Namespace:   name.Namespace,
			Labels:      pObj.Labels,
			Annotations: map[string]string{},
		},
		Spec: certmanagerv1.CertificateSpec{},
	}
	for k, v := range pObj.Annotations {
		vCertificate.Annotations[k] = v
	}
	vCertificate.Annotations[constants.BackwardSyncAnnotation] = "true"

	// rewrite spec
	vCertificateSpec, err := s.rewriteSpecBackwards(ctx, &pObj.Spec, name)
	if err != nil {
		return nil, err
	}

	vCertificate.Spec = *vCertificateSpec
	return vCertificate, nil
}

func (s *certificateSyncer) translateUpdateBackwards(ctx *synccontext.SyncContext, pObj, vObj *certmanagerv1.Certificate) (*certmanagerv1.Certificate, error) {
	var updated *certmanagerv1.Certificate

	// check annotations & labels
	if !equality.Semantic.DeepEqual(pObj.Labels, vObj.Labels) {
		updated = newIfNil(updated, vObj)
		updated.Labels = pObj.Labels
	}

	// check annotations
	newAnnotations := map[string]string{}
	for k, v := range pObj.Annotations {
		newAnnotations[k] = v
	}
	newAnnotations[constants.BackwardSyncAnnotation] = "true"
	if !equality.Semantic.DeepEqual(newAnnotations, vObj.Annotations) {
		updated = newIfNil(updated, vObj)
		updated.Annotations = newAnnotations
	}

	// update spec
	vSpec, err := s.rewriteSpecBackwards(ctx, &pObj.Spec, types.NamespacedName{Namespace: vObj.Namespace, Name: vObj.Name})
	if err != nil {
		return nil, err
	}
	if !equality.Semantic.DeepEqual(*vSpec, vObj.Spec) {
		updated = newIfNil(updated, vObj)
		updated.Spec = *vSpec
	}

	return updated, nil
}

func (s *certificateSyncer) rewriteSpecBackwards(ctx *synccontext.SyncContext, pObjSpec *certmanagerv1.CertificateSpec, vName types.NamespacedName) (*certmanagerv1.CertificateSpec, error) {
	vObjSpec := pObjSpec.DeepCopy()

	// find issuer
	vObjSpec.SecretName = vName.Name
	if vObjSpec.IssuerRef.Kind == "Issuer" {
		vIssuerName := mappings.HostToVirtual(ctx, vName.Name, vName.Namespace, nil, certmanagerv1.SchemeGroupVersion.WithKind("Issuer"))
		vObjSpec.IssuerRef.Name = vIssuerName.Name
	}

	return vObjSpec, nil
}

func newIfNil(updated *certmanagerv1.Certificate, pObj *certmanagerv1.Certificate) *certmanagerv1.Certificate {
	if updated == nil {
		return pObj.DeepCopy()
	}
	return updated
}
