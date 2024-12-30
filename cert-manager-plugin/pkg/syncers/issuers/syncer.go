package issuers

import (
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/loft-sh/vcluster/pkg/mappings/generic"
	"github.com/loft-sh/vcluster/pkg/patcher"
	"github.com/loft-sh/vcluster/pkg/syncer"
	context "github.com/loft-sh/vcluster/pkg/syncer/synccontext"
	"github.com/loft-sh/vcluster/pkg/syncer/translator"
	syncertypes "github.com/loft-sh/vcluster/pkg/syncer/types"
	"github.com/loft-sh/vcluster/pkg/util/translate"
	"k8s.io/apimachinery/pkg/api/equality"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(ctx *context.RegisterContext) (syncertypes.Syncer, error) {
	_, _, err := translate.EnsureCRDFromPhysicalCluster(ctx.Context, ctx.PhysicalManager.GetConfig(), ctx.VirtualManager.GetConfig(), certmanagerv1.SchemeGroupVersion.WithKind("Issuer"))
	if err != nil {
		return nil, err
	}
	mapper, err := generic.NewMapper(ctx, &certmanagerv1.Issuer{}, translate.Default.HostName)
	if err != nil {
		return nil, err
	}
	return &issuerSyncer{
		GenericTranslator: translator.NewGenericTranslator(ctx, "issuer", &certmanagerv1.Issuer{}, mapper),
	}, nil
}

type issuerSyncer struct {
	syncertypes.GenericTranslator
}

var _ syncertypes.Syncer = &issuerSyncer{}

func (s *issuerSyncer) Syncer() syncertypes.Sync[client.Object] {
	return syncer.ToGenericSyncer(s)
}

func (s *issuerSyncer) SyncToHost(ctx *context.SyncContext, evt *context.SyncToHostEvent[*certmanagerv1.Issuer]) (ctrl.Result, error) {
	return patcher.CreateHostObject(ctx, evt.Virtual, s.translate(evt.Virtual), s.EventRecorder(), false)
}

func (s *issuerSyncer) Sync(ctx *context.SyncContext, event *context.SyncEvent[*certmanagerv1.Issuer]) (ctrl.Result, error) {
	vIssuer := event.Virtual
	pIssuer := event.Host

	if !equality.Semantic.DeepEqual(vIssuer.Status, pIssuer.Status) {
		newIssuer := vIssuer.DeepCopy()
		newIssuer.Status = pIssuer.Status
		ctx.Log.Infof("update virtual issuer %s/%s, because status is out of sync", vIssuer.Namespace, vIssuer.Name)
		err := ctx.VirtualClient.Status().Update(ctx.Context, newIssuer)
		if err != nil {
			return ctrl.Result{}, err
		}

		// we will requeue anyways
		return ctrl.Result{}, nil
	}

	s.translateUpdate(pIssuer, vIssuer)

	return ctrl.Result{}, nil
}

func (s *issuerSyncer) SyncToVirtual(ctx *context.SyncContext, event *context.SyncToVirtualEvent[*certmanagerv1.Issuer]) (_ ctrl.Result, retErr error) {
	// TODO: Do we need to ensure that there are no references to the issuer?
	return patcher.DeleteHostObject(ctx, event.Host, event.VirtualOld, "virtual object was deleted")
}
