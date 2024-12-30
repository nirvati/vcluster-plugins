package secrets

import (
	"fmt"

	"github.com/loft-sh/vcluster/pkg/mappings"
	"github.com/loft-sh/vcluster/pkg/patcher"
	"github.com/loft-sh/vcluster/pkg/syncer"
	context "github.com/loft-sh/vcluster/pkg/syncer/synccontext"
	"github.com/loft-sh/vcluster/pkg/syncer/translator"
	syncertypes "github.com/loft-sh/vcluster/pkg/syncer/types"
	"github.com/loft-sh/vcluster/pkg/util/translate"
	"github.com/nirvati/vcluster-cert-manager-plugin/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func New(ctx *context.RegisterContext) (syncertypes.Object, error) {
	mapper, err := ctx.Mappings.ByGVK(mappings.Secrets())
	if err != nil {
		return nil, err
	}
	return &secretSyncer{
		GenericTranslator: translator.NewGenericTranslator(ctx, "secret", &corev1.Secret{}, mapper),

		virtualClient:  ctx.VirtualManager.GetClient(),
		physicalClient: ctx.PhysicalManager.GetClient(),
	}, nil
}

type secretSyncer struct {
	syncertypes.GenericTranslator

	virtualClient  client.Client
	physicalClient client.Client
}

func (s *secretSyncer) Object() client.Object {
	return &corev1.Secret{}
}

func (s *secretSyncer) SyncToHost(ctx *context.SyncContext, evt *context.SyncToHostEvent[*corev1.Secret]) (ctrl.Result, error) {
	// was secret created by certificate or issuer?
	shouldSync, _ := s.shouldSyncBackwards(nil, evt.Virtual)
	if shouldSync {
		// delete here as secret is no longer needed
		ctx.Log.Infof("delete virtual secret %s/%s, because physical got deleted", evt.Virtual.GetNamespace(), evt.Virtual.GetName())
		return ctrl.Result{}, ctx.VirtualClient.Delete(ctx.Context, evt.Virtual)
	}

	// is secret used by an issuer or certificate?
	createNeeded, err := s.shouldSyncForward(ctx, evt.Virtual)
	if err != nil {
		return ctrl.Result{}, err
	} else if !createNeeded {
		return ctrl.Result{}, s.removeController(ctx, evt.Virtual)
	}

	// switch controller
	switched, err := s.switchController(ctx, evt.Virtual)
	if err != nil {
		return ctrl.Result{}, err
	} else if switched {
		return ctrl.Result{}, nil
	}

	// create the secret if it's needed
	return patcher.CreateHostObject(ctx, evt.Virtual, s.translate(ctx, evt.Virtual), s.EventRecorder(), true)
}

func (s *secretSyncer) Sync(ctx *context.SyncContext, evt *context.SyncEvent[*corev1.Secret]) (_ ctrl.Result, retErr error) {
	// was secret created by certificate or issuer?
	shouldSyncBackwards, _ := s.shouldSyncBackwards(evt.Host, evt.Virtual)
	if shouldSyncBackwards {
		// delete here as secret is no longer needed
		if equality.Semantic.DeepEqual(evt.Host.Data, evt.Virtual.Data) && evt.Virtual.Type == evt.Host.Type {
			return ctrl.Result{}, nil
		}

		// update secret if necessary
		evt.Virtual.Data = evt.Host.Data
		evt.Virtual.Type = evt.Host.Type
		ctx.Log.Infof("update virtual secret %s/%s because physical secret has changed", evt.Virtual.Namespace, evt.Virtual.Name)
		return ctrl.Result{}, ctx.VirtualClient.Delete(ctx.Context, evt.Virtual)
	}

	// is secret used by an issuer or certificate?
	used, err := s.shouldSyncForward(ctx, evt.Virtual)
	if err != nil {
		return ctrl.Result{}, err
	} else if !used {
		return ctrl.Result{}, s.removeController(ctx, evt.Virtual)
	}

	// switch controller
	switched, err := s.switchController(ctx, evt.Virtual)
	if err != nil {
		return ctrl.Result{}, err
	} else if switched {
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
	s.translateUpdate(evt.Host, evt.Virtual)

	return ctrl.Result{}, nil
}

var _ syncertypes.Syncer = &secretSyncer{}

func (s *secretSyncer) Syncer() syncertypes.Sync[client.Object] {
	return syncer.ToGenericSyncer[*corev1.Secret](s)
}

func (s *secretSyncer) SyncToVirtual(ctx *context.SyncContext, evt *context.SyncToVirtualEvent[*corev1.Secret]) (ctrl.Result, error) {
	// was secret created by certificate or issuer?
	shouldSyncBackwards, vName := s.shouldSyncBackwards(evt.Host, nil)
	if shouldSyncBackwards {
		vSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        vName.Name,
				Namespace:   vName.Namespace,
				Annotations: map[string]string{},
				Labels:      map[string]string{},
			},
			Data: evt.Host.Data,
			Type: evt.Host.Type,
		}
		for k, v := range evt.Host.Annotations {
			vSecret.Annotations[k] = v
		}
		for k, v := range evt.Host.Labels {
			vSecret.Labels[k] = v
		}
		vSecret.Annotations[constants.BackwardSyncAnnotation] = "true"
		vSecret.Labels[translate.ControllerLabel] = constants.PluginName
		ctx.Log.Infof("create virtual secret %s/%s because physical secret exists", vSecret.Namespace, vSecret.Name)
		return ctrl.Result{}, ctx.VirtualClient.Create(ctx.Context, vSecret)
	}

	// don't do anything here
	return ctrl.Result{}, nil
}

func (s *secretSyncer) removeController(ctx *context.SyncContext, vSecret *corev1.Secret) error {
	// remove us as owner
	if vSecret.Labels != nil && vSecret.Labels[translate.ControllerLabel] == constants.PluginName {
		delete(vSecret.Labels, translate.ControllerLabel)
		ctx.Log.Infof("update secret %s/%s because we the controlling party, but secret is not needed anymore", vSecret.Namespace, vSecret.Name)
		return ctx.VirtualClient.Update(ctx.Context, vSecret)
	}

	return nil
}

func (s *secretSyncer) switchController(ctx *context.SyncContext, vSecret *corev1.Secret) (bool, error) {
	// check if we own the secret
	if vSecret.Labels == nil || vSecret.Labels[translate.ControllerLabel] == "" {
		if vSecret.Labels == nil {
			vSecret.Labels = map[string]string{}
		}
		vSecret.Labels[translate.ControllerLabel] = constants.PluginName
		ctx.Log.Infof("update secret %s/%s because we are not the controlling party", vSecret.Namespace, vSecret.Name)
		return true, ctx.VirtualClient.Update(ctx.Context, vSecret)
	} else if vSecret.Labels[translate.ControllerLabel] != constants.PluginName {
		return true, nil
	}

	return false, nil
}
