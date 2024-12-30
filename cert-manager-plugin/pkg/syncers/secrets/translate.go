package secrets

import (
	"github.com/loft-sh/vcluster/pkg/syncer/synccontext"
	"github.com/loft-sh/vcluster/pkg/util/translate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *secretSyncer) translate(ctx *synccontext.SyncContext, vObj *corev1.Secret) *corev1.Secret {
	newSecret := s.TranslateMetadata(ctx, vObj).(*corev1.Secret)
	if newSecret.Type == corev1.SecretTypeServiceAccountToken {
		newSecret.Type = corev1.SecretTypeOpaque
	}

	return newSecret
}

func (s *secretSyncer) TranslateMetadata(ctx *synccontext.SyncContext, pObj client.Object) client.Object {
	vObj := pObj.DeepCopyObject().(client.Object)
	vObj.SetResourceVersion("")
	vObj.SetUID("")
	vObj.SetManagedFields(nil)
	vObj.SetOwnerReferences(nil)
	vObj.SetFinalizers(nil)
	vObj.SetAnnotations(s.updateVirtualAnnotations(vObj.GetAnnotations()))
	nn := s.HostToVirtual(ctx, types.NamespacedName{Name: pObj.GetName(), Namespace: pObj.GetNamespace()}, pObj)
	vObj.SetName(nn.Name)
	vObj.SetNamespace(nn.Namespace)
	return vObj
}

func (s *secretSyncer) updateVirtualAnnotations(a map[string]string) map[string]string {
	if a == nil {
		return map[string]string{translate.ControllerLabel: s.Name()}
	}

	a[translate.ControllerLabel] = s.Name()
	delete(a, translate.NameAnnotation)
	delete(a, translate.NamespaceAnnotation)
	delete(a, translate.UIDAnnotation)
	delete(a, translate.KindAnnotation)
	delete(a, translate.HostNameAnnotation)
	delete(a, translate.HostNamespaceAnnotation)
	delete(a, corev1.LastAppliedConfigAnnotation)
	return a
}

func (s *secretSyncer) translateUpdate(pObj, vObj *corev1.Secret) {
	pObj.Data = vObj.Data
	pObj.Type = vObj.Type

	// check annotations
	pObj.Labels = translate.HostLabels(vObj, pObj)
	pObj.Annotations = translate.HostAnnotations(vObj, pObj)
}
