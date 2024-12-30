package certificates

import (
	context2 "context"
	"strings"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/loft-sh/vcluster/pkg/mappings/generic"
	"github.com/loft-sh/vcluster/pkg/syncer/synccontext"
	context "github.com/loft-sh/vcluster/pkg/syncer/synccontext"
	syncertypes "github.com/loft-sh/vcluster/pkg/syncer/types"
	"github.com/loft-sh/vcluster/pkg/util/clienthelper"
	"github.com/loft-sh/vcluster/pkg/util/translate"
	"github.com/nirvati/vcluster-cert-manager-plugin/pkg/constants"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	IndexByIngressCertificate = "indexbyingresscertificate"
)

type certificateMapper struct {
	synccontext.Mapper

	virtualClient client.Client
}

func CreateCertificateMapper(ctx *synccontext.RegisterContext) (synccontext.Mapper, error) {
	_, _, err := translate.EnsureCRDFromPhysicalCluster(ctx.Context, ctx.PhysicalManager.GetConfig(), ctx.VirtualManager.GetConfig(), certmanagerv1.SchemeGroupVersion.WithKind("Certificate"))

	if err != nil {
		return nil, err
	}
	mapper, err := generic.NewMapperWithoutRecorder(ctx, &certmanagerv1.Certificate{}, func(ctx *synccontext.SyncContext, vName, vNamespace string, _ client.Object) types.NamespacedName {
		return translate.Default.HostName(ctx, vName, vNamespace)
	})
	if err != nil {
		return nil, err
	}

	return generic.WithRecorder(&certificateMapper{
		Mapper:        mapper,
		virtualClient: ctx.VirtualManager.GetClient(),
	}), nil
}

var _ syncertypes.IndicesRegisterer = &certificateMapper{}

func (s *certificateMapper) RegisterIndices(ctx *context.RegisterContext) error {
	return ctx.VirtualManager.GetFieldIndexer().IndexField(ctx.Context, &networkingv1.Ingress{}, IndexByIngressCertificate, func(rawObj client.Object) []string {
		return certificateNamesFromIngress(rawObj.(*networkingv1.Ingress))
	})
}

var _ syncertypes.ControllerModifier = &certificateMapper{}

func (s *certificateMapper) ModifyController(ctx *context.RegisterContext, builder *builder.Builder) (*builder.Builder, error) {
	builder = builder.Watches(&networkingv1.Ingress{}, handler.EnqueueRequestsFromMapFunc(mapIngresses))
	return builder, nil
}

func (s *certificateMapper) nameByIngress(pObj client.Object) types.NamespacedName {
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

func (s *certificateMapper) HostToVirtual(ctx *synccontext.SyncContext, req types.NamespacedName, pObj client.Object) types.NamespacedName {
	namespacedName := s.Mapper.HostToVirtual(ctx, req, pObj)
	if namespacedName.Name != "" {
		return namespacedName
	}

	namespacedName = s.nameByIngress(pObj)
	if namespacedName.Name != "" {
		return namespacedName
	}

	return types.NamespacedName{}
}

func certificateNamesFromIngress(ingress *networkingv1.Ingress) []string {
	certificates := []string{}

	// Do not include certificate.Spec.SecretName here as this will be handled separately by a different controller
	if ingress.Annotations != nil && (ingress.Annotations[constants.IssuerAnnotation] != "" || ingress.Annotations[constants.ClusterIssuerAnnotation] != "") {
		for _, secret := range ingress.Spec.TLS {
			if secret.SecretName == "" {
				continue
			}

			certificates = append(certificates, translate.Default.HostName(nil, secret.SecretName, ingress.Namespace).Name)
			certificates = append(certificates, ingress.Namespace+"/"+secret.SecretName)
		}
	}
	return certificates
}

func mapIngresses(ctx context2.Context, obj client.Object) []reconcile.Request {
	ingress, ok := obj.(*networkingv1.Ingress)
	if !ok {
		return nil
	}

	requests := []reconcile.Request{}
	names := certificateNamesFromIngress(ingress)
	for _, name := range names {
		splitted := strings.Split(name, "/")
		if len(splitted) == 2 {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: splitted[0],
					Name:      splitted[1],
				},
			})
		}
	}

	return requests
}
