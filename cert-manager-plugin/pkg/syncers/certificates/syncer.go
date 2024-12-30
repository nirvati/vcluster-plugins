package certificates

import (
	context2 "context"
	"fmt"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/loft-sh/vcluster/pkg/patcher"
	"github.com/loft-sh/vcluster/pkg/syncer"
	"github.com/loft-sh/vcluster/pkg/syncer/synccontext"
	"github.com/loft-sh/vcluster/pkg/syncer/translator"
	syncertypes "github.com/loft-sh/vcluster/pkg/syncer/types"
	"github.com/loft-sh/vcluster/pkg/util/clienthelper"
	"github.com/loft-sh/vcluster/pkg/util/translate"
	"github.com/nirvati/vcluster-cert-manager-plugin/pkg/constants"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(ctx *synccontext.RegisterContext) (syncertypes.Syncer, error) {
	mapper, err := CreateCertificateMapper(ctx)
	if err != nil {
		return nil, err
	}
	return &certificateSyncer{
		GenericTranslator: translator.NewGenericTranslator(ctx, "certificate", &certmanagerv1.Certificate{}, mapper),

		virtualClient: ctx.VirtualManager.GetClient(),
	}, nil
}

type certificateSyncer struct {
	syncertypes.GenericTranslator

	virtualClient client.Client
}

func (f *certificateSyncer) Syncer() syncertypes.Sync[client.Object] {
	return syncer.ToGenericSyncer[*certmanagerv1.Certificate](f)
}

func (s *certificateSyncer) SyncToHost(ctx *synccontext.SyncContext, evt *synccontext.SyncToHostEvent[*certmanagerv1.Certificate]) (ctrl.Result, error) {
	// was certificate created by ingress?
	shouldSync, _ := s.shouldSyncBackwards(nil, evt.Virtual)
	if shouldSync {
		// delete here as certificate is no longer needed
		ctx.Log.Infof("delete virtual certificate %s/%s, because physical got deleted", evt.Virtual.GetNamespace(), evt.Virtual.GetName())
		return ctrl.Result{}, ctx.VirtualClient.Delete(ctx.Context, evt.Virtual)
	}

	return patcher.CreateHostObject(ctx, evt.Virtual, s.translate(ctx, evt.Virtual), s.EventRecorder(), true)
}

func (s *certificateSyncer) Sync(ctx *synccontext.SyncContext, evt *synccontext.SyncEvent[*certmanagerv1.Certificate]) (_ ctrl.Result, retErr error) {
	if !equality.Semantic.DeepEqual(evt.Virtual.Status, evt.Host.Status) {
		newIssuer := evt.Virtual.DeepCopy()
		newIssuer.Status = evt.Host.Status
		ctx.Log.Infof("update virtual certificate %s/%s, because status is out of sync", evt.Virtual.Namespace, evt.Virtual.Name)
		err := ctx.VirtualClient.Status().Update(ctx.Context, newIssuer)
		if err != nil {
			return ctrl.Result{}, err
		}

		// we will requeue anyways
		return ctrl.Result{}, nil
	}

	// was certificate created by ingress?
	shouldSync, _ := s.shouldSyncBackwards(evt.Host, evt.Virtual)
	if shouldSync {
		updated, err := s.translateUpdateBackwards(ctx, evt.Host, evt.Virtual)
		if err != nil {
			return ctrl.Result{}, err
		}
		if updated != nil {
			ctx.Log.Infof("update virtual certificate %s/%s, because spec is out of sync", evt.Virtual.Namespace, evt.Virtual.Name)
			return ctrl.Result{}, s.virtualClient.Update(ctx.Context, updated)
		}

		return ctrl.Result{}, nil
	}

	patchHelper, err := patcher.NewSyncerPatcher(ctx, evt.Host, evt.Virtual)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("new syncer patcher: %w", err)
	}

	defer func() {
		if err := patchHelper.Patch(ctx, evt.Host, evt.Virtual); err != nil {
			retErr = errors.NewAggregate([]error{retErr, err})
		}
		if retErr != nil {
			s.EventRecorder().Eventf(evt.Virtual, "Warning", "SyncError", "Error syncing: %v", retErr)
		}
	}()

	// any changes made below here are correctly synced
	s.translateUpdate(ctx, evt)

	return ctrl.Result{}, nil
}

var _ syncertypes.Syncer = &certificateSyncer{}

func (s *certificateSyncer) shouldSyncBackwards(pCertificate, vCertificate *certmanagerv1.Certificate) (bool, types.NamespacedName) {
	// we sync secrets that were generated from certificates or issuers into the vcluster
	if vCertificate != nil && vCertificate.Annotations != nil && vCertificate.Annotations[constants.BackwardSyncAnnotation] == "true" {
		return true, types.NamespacedName{
			Namespace: vCertificate.Namespace,
			Name:      vCertificate.Name,
		}
	} else if pCertificate == nil {
		return false, types.NamespacedName{}
	}

	name := s.nameByIngress(pCertificate)
	if name.Name != "" {
		return true, name
	}

	return false, types.NamespacedName{}
}

// TODO: This function is duplicated and also exists in the mapper
func (s *certificateSyncer) nameByIngress(pObj client.Object) types.NamespacedName {
	vIngress := &networkingv1.Ingress{}
	err := clienthelper.GetByIndex(context2.TODO(), s.virtualClient, vIngress, IndexByIngressCertificate, pObj.GetName())
	if err == nil && vIngress.Name != "" {
		for _, secret := range vIngress.Spec.TLS {
			if translate.Default.HostName(nil, secret.SecretName, vIngress.Namespace).Name == pObj.GetName() {
				return types.NamespacedName{
					Name:      secret.SecretName,
					Namespace: vIngress.Namespace,
				}
			}
		}
	}

	return types.NamespacedName{}
}

func (s *certificateSyncer) SyncToVirtual(ctx *synccontext.SyncContext, evt *synccontext.SyncToVirtualEvent[*certmanagerv1.Certificate]) (ctrl.Result, error) {
	// was certificate created by ingress?
	shouldSync, vName := s.shouldSyncBackwards(evt.Host, nil)
	if shouldSync {
		ctx.Log.Infof("create virtual certificate %s/%s, because physical is there and virtual is missing", vName.Namespace, vName.Name)
		vCertificate, err := s.translateBackwards(ctx, evt.Host, vName)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, s.virtualClient.Create(ctx.Context, vCertificate)
	}

	managed, err := s.IsManaged(ctx, evt.Host)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !managed {
		return ctrl.Result{}, nil
	}
	return patcher.DeleteHostObject(ctx, evt.Host, evt.VirtualOld, "virtual object was deleted")
}

func (s *certificateSyncer) GroupVersionKind() schema.GroupVersionKind {
	return certmanagerv1.SchemeGroupVersion.WithKind("Certificate")
}
