package gangpack

import (
	"context"
	"time"

	"k8s.io/client-go/rest"
	schedv1alpha1 "sigs.k8s.io/scheduler-plugins/apis/scheduling/v1alpha1"
	schedclient "sigs.k8s.io/scheduler-plugins/pkg/generated/clientset/versioned"
	schedinformers "sigs.k8s.io/scheduler-plugins/pkg/generated/informers/externalversions"
)

// informerReader is the live podGroupReader: it resolves a pod's PodGroup facts
// from a scheduler-plugins PodGroup informer cache.
type informerReader struct {
	// getPG fetches a PodGroup by namespace/name (from the informer lister). A
	// func rather than the lister interface so the read path is unit-testable
	// without standing up an informer.
	getPG func(namespace, name string) (*schedv1alpha1.PodGroup, error)
	// defaultPermitTimeout is supplied by plugin configuration. Zero means no
	// fallback; the PodGroup must then declare ScheduleTimeoutSeconds itself.
	defaultPermitTimeout  time.Duration
	topologyKeyAnnotation string
	placementGroupLabel   string
}

var _ podGroupReader = &informerReader{}

// factsFromPodGroup extracts the placement facts the plugin needs from a
// PodGroup: the gang size (Spec.MinMember), the domain topology key the workload
// declared (using the configured annotation key), and the gate timeout (the standard
// Spec.ScheduleTimeoutSeconds, falling back to the configured value when unset).
// A PodGroup without the topology annotation yields an empty key, which
// resolveGang treats as unresolvable (no domain to pin to).
func factsFromPodGroup(pg *schedv1alpha1.PodGroup, topologyKeyAnnotation string, defaultPermitTimeout time.Duration) (minMember int, topologyKey string, timeout time.Duration) {
	if pg == nil {
		return 0, "", defaultPermitTimeout
	}
	timeout = defaultPermitTimeout
	if s := pg.Spec.ScheduleTimeoutSeconds; s != nil && *s > 0 {
		timeout = time.Duration(*s) * time.Second
	}
	return int(pg.Spec.MinMember), pg.Annotations[topologyKeyAnnotation], timeout
}

// get resolves a PodGroup into (minMember, topologyKey, timeout, uid, found). Any
// lookup error or missing PodGroup is reported as not-found; PreFilter holds a
// labeled member until the informer can validate the group.
func (r *informerReader) get(namespace, name string) (minMember int, topologyKey string, timeout time.Duration, uid string, found bool) {
	pg, err := r.getPG(namespace, name)
	if err != nil || pg == nil {
		return 0, "", 0, "", false
	}
	mm, tk, to := factsFromPodGroup(pg, r.topologyKeyAnnotation, r.defaultPermitTimeout)
	return mm, tk, to, string(pg.UID), true
}

func (r *informerReader) placementGroup(namespace, name string) (string, bool) {
	pg, err := r.getPG(namespace, name)
	if err != nil || pg == nil {
		return "", false
	}
	if r.placementGroupLabel == "" {
		return "", false
	}
	value, present := pg.Labels[r.placementGroupLabel]
	return value, present
}

// newInformerReader builds a PodGroup informer from the scheduler's shared
// kubeconfig, starts it, and returns a reader backed by its lister. ctx bounds
// the informer's lifetime (the scheduler's).
//
// The PodGroup CRD is an OPTIONAL dependency: it is the ecosystem-standard gang
// API, but the scheduler must still boot without it and schedule non-gang pods
// normally — gang behavior activates once the CRD and its PodGroups exist and the
// informer syncs. So the initial cache sync is best-effort and time-bounded: when
// the CRD is present it returns as soon as the cache syncs (sub-second); when it
// is absent we stop waiting after the configured sync timeout rather than
// hanging New() forever. With no configured timeout, startup does not block.
// The informer keeps retrying under ctx, so a CRD installed later is picked up.
func newInformerReader(ctx context.Context, cfg *rest.Config, topologyKeyAnnotation, placementGroupLabel string, defaultPermitTimeout, syncTimeout time.Duration) (*informerReader, error) {
	client, err := schedclient.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	factory := schedinformers.NewSharedInformerFactory(client, 0)
	lister := factory.Scheduling().V1alpha1().PodGroups().Lister()
	factory.Start(ctx.Done())

	if syncTimeout > 0 {
		syncCtx, cancel := context.WithTimeout(ctx, syncTimeout)
		defer cancel()
		factory.WaitForCacheSync(syncCtx.Done())
	}

	return &informerReader{
		defaultPermitTimeout:  defaultPermitTimeout,
		topologyKeyAnnotation: topologyKeyAnnotation,
		placementGroupLabel:   placementGroupLabel,
		getPG: func(namespace, name string) (*schedv1alpha1.PodGroup, error) {
			return lister.PodGroups(namespace).Get(name)
		},
	}, nil
}
