package collectors

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/red-hat-storage/ocs-operator/metrics/v4/internal/options"
	cephv1 "github.com/rook/rook/pkg/apis/ceph.rook.io/v1"
	cephv1listers "github.com/rook/rook/pkg/client/listers/ceph.rook.io/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

var (
	mockCephBlockPool1 = cephv1.CephBlockPool{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "ceph.rook.io/v1",
			Kind:       "CephBlockPool",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mockCephBlockPool-1",
			Namespace: "openshift-storage",
		},
		Spec: cephv1.NamedBlockPoolSpec{
			PoolSpec: cephv1.PoolSpec{
				Mirroring: cephv1.MirroringSpec{Enabled: true},
			},
		},
		Status: &cephv1.CephBlockPoolStatus{},
	}
	mockCephBlockPool2 = cephv1.CephBlockPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mockCephBlockPool-2",
			Namespace: "openshift-storage",
		},
		Spec:   cephv1.NamedBlockPoolSpec{},
		Status: &cephv1.CephBlockPoolStatus{},
	}
	mockCephBlockPoolRadosNamespace1 = cephv1.CephBlockPoolRadosNamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mockCephBlockPoolRadosNamespace",
			Namespace: "openshift-storage",
		},
		Spec: cephv1.CephBlockPoolRadosNamespaceSpec{
			BlockPoolName: "mockCephBlockPool-1",
			Mirroring: &cephv1.RadosNamespaceMirroring{
				Mode: cephv1.RadosNamespaceMirroringModePool,
			},
		},
		Status: &cephv1.CephBlockPoolRadosNamespaceStatus{},
	}
)

func (c *CephBlockPoolCollector) GetInformer() cache.SharedIndexInformer {
	return c.Informer
}

func getMockCephBlockPoolCollector(t *testing.T, mockOpts *options.Options) (mockCephBlockPoolCollector *CephBlockPoolCollector) {
	setKubeConfig(t)
	mockCephBlockPoolCollector = NewCephBlockPoolCollector(mockOpts)
	assert.NotNil(t, mockCephBlockPoolCollector)
	return
}

func TestNewCephBlockPoolCollector(t *testing.T) {
	tests := Tests{
		{
			name: "Test CephBlockPoolCollector",
			args: args{
				opts: mockOpts,
			},
		},
	}
	for _, tt := range tests {
		got := getMockCephBlockPoolCollector(t, tt.args.opts)
		assert.NotNil(t, got.AllowedNamespaces)
		assert.NotNil(t, got.Informer)
	}
}

func TestGetAllBlockPools(t *testing.T) {
	mockOpts.StopCh = make(chan struct{})
	defer close(mockOpts.StopCh)

	cephBlockPoolCollector := getMockCephBlockPoolCollector(t, mockOpts)

	tests := Tests{
		{
			name: "CephBlockPools doesn't exist",
			args: args{
				lister:     cephv1listers.NewCephBlockPoolLister(cephBlockPoolCollector.Informer.GetIndexer()),
				namespaces: cephBlockPoolCollector.AllowedNamespaces,
			},
			inputObjects: []runtime.Object{},
			// []*cephv1.CephBlockPool(nil) is not DeepEqual to []*cephv1.CephBlockPool{}
			// getAllBlockPools returns []*cephv1.CephBlockPool(nil) if no CephBlockPool is present
			wantObjects: []runtime.Object(nil),
		},
		{
			name: "One CephBlockPools exists",
			args: args{
				lister:     cephv1listers.NewCephBlockPoolLister(cephBlockPoolCollector.Informer.GetIndexer()),
				namespaces: cephBlockPoolCollector.AllowedNamespaces,
			},
			inputObjects: []runtime.Object{
				&mockCephBlockPool1,
			},
			wantObjects: []runtime.Object{
				&mockCephBlockPool1,
			},
		},
		{
			name: "Two CephBlockPools exists",
			args: args{
				lister:     cephv1listers.NewCephBlockPoolLister(cephBlockPoolCollector.Informer.GetIndexer()),
				namespaces: cephBlockPoolCollector.AllowedNamespaces,
			},
			inputObjects: []runtime.Object{
				&mockCephBlockPool1,
				&mockCephBlockPool2,
			},
			wantObjects: []runtime.Object{
				&mockCephBlockPool1,
				&mockCephBlockPool2,
			},
		},
	}
	for _, tt := range tests {
		setInformer(t, tt.inputObjects, cephBlockPoolCollector)
		gotCephBlockPools := getAllBlockPools(tt.args.lister.(cephv1listers.CephBlockPoolLister), tt.args.namespaces)
		assert.Len(t, gotCephBlockPools, len(tt.wantObjects))
		for _, obj := range gotCephBlockPools {
			assert.Contains(t, tt.wantObjects, obj)
		}
		resetInformer(t, tt.inputObjects, cephBlockPoolCollector)
	}
}

func TestCollectPoolMirroringImageHealth(t *testing.T) {
	mockOpts.StopCh = make(chan struct{})
	defer close(mockOpts.StopCh)

	cephBlockPoolCollector := getMockCephBlockPoolCollector(t, mockOpts)

	objOk := mockCephBlockPool1.DeepCopy()
	objOk.Name = objOk.Name + "ok"
	objOk.Status = &cephv1.CephBlockPoolStatus{
		MirroringStatus: &cephv1.MirroringStatusSpec{
			MirroringStatus: cephv1.MirroringStatus{
				Summary: &cephv1.MirroringStatusSummarySpec{
					ImageHealth: "OK",
				},
			},
		},
	}

	objUnknown := mockCephBlockPool1.DeepCopy()
	objUnknown.Name = objUnknown.Name + "unknown"
	objUnknown.Status = &cephv1.CephBlockPoolStatus{
		MirroringStatus: &cephv1.MirroringStatusSpec{
			MirroringStatus: cephv1.MirroringStatus{
				Summary: &cephv1.MirroringStatusSummarySpec{
					ImageHealth: "UNKNOWN",
				},
			},
		},
	}

	objWarning := mockCephBlockPool1.DeepCopy()
	objWarning.Name = objWarning.Name + "warning"
	objWarning.Status = &cephv1.CephBlockPoolStatus{
		MirroringStatus: &cephv1.MirroringStatusSpec{
			MirroringStatus: cephv1.MirroringStatus{
				Summary: &cephv1.MirroringStatusSummarySpec{
					ImageHealth: "WARNING",
				},
			},
		},
	}

	objError := mockCephBlockPool1.DeepCopy()
	objError.Name = objError.Name + "error"
	objError.Status = &cephv1.CephBlockPoolStatus{
		MirroringStatus: &cephv1.MirroringStatusSpec{
			MirroringStatus: cephv1.MirroringStatus{
				Summary: &cephv1.MirroringStatusSummarySpec{
					ImageHealth: "ERROR",
				},
			},
		},
	}

	tests := Tests{
		{
			name: "Collect CephBlockPool mirroring image health metrics",
			args: args{
				objects: []runtime.Object{
					objOk,
					objUnknown,
					objWarning,
					objError,
				},
			},
		},
		{
			name: "Empty CephBlockPool",
			args: args{
				objects: []runtime.Object{},
			},
		},
	}
	// Image health
	for _, tt := range tests {
		ch := make(chan prometheus.Metric)
		metric := dto.Metric{}
		go func() {
			var cephBlockPools []*cephv1.CephBlockPool
			for _, obj := range tt.args.objects {
				cephBlockPools = append(cephBlockPools, obj.(*cephv1.CephBlockPool))
			}
			cephBlockPoolCollector.collectMirroringImageHealth(cephBlockPools, ch)
			close(ch)
		}()

		for m := range ch {
			assert.Contains(t, m.Desc().String(), "image_health")
			metric.Reset()
			err := m.Write(&metric)
			assert.Nil(t, err)
			labels := metric.GetLabel()
			for _, label := range labels {
				if *label.Name == "name" {
					if *label.Value == objOk.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(0))
					} else if *label.Value == objUnknown.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(1))
					} else if *label.Value == objWarning.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(2))
					} else if *label.Value == objError.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(3))
					}
				} else if *label.Name == "namespace" {
					assert.Contains(t, cephBlockPoolCollector.AllowedNamespaces, *label.Value)
				} else if *label.Name == "rados_namespace" {
					assert.Contains(t, defaultRadosNamespace, *label.Value)
				}
			}
		}
	}
}

func TestCollectPoolMirroringStatus(t *testing.T) {
	mockOpts.StopCh = make(chan struct{})
	defer close(mockOpts.StopCh)

	cephBlockPoolCollector := getMockCephBlockPoolCollector(t, mockOpts)

	objEnabled := mockCephBlockPool1.DeepCopy()
	objEnabled.Name = objEnabled.Name + "enabled"
	objEnabled.Spec = cephv1.NamedBlockPoolSpec{
		PoolSpec: cephv1.PoolSpec{
			Mirroring: cephv1.MirroringSpec{Enabled: true},
		},
	}

	objDisabled := mockCephBlockPool1.DeepCopy()
	objDisabled.Name = objDisabled.Name + "disabled"
	objDisabled.Spec = cephv1.NamedBlockPoolSpec{
		PoolSpec: cephv1.PoolSpec{
			Mirroring: cephv1.MirroringSpec{Enabled: false},
		},
	}

	tests := Tests{
		{
			name: "Collect CephBlockPool mirroring status",
			args: args{
				objects: []runtime.Object{
					objEnabled,
					objDisabled,
				},
			},
		},
		{
			name: "Empty CephBlockPool",
			args: args{
				objects: []runtime.Object{},
			},
		},
	}
	// Image health
	for _, tt := range tests {
		ch := make(chan prometheus.Metric)
		metric := dto.Metric{}
		go func() {
			var cephBlockPools []*cephv1.CephBlockPool
			for _, obj := range tt.args.objects {
				cephBlockPools = append(cephBlockPools, obj.(*cephv1.CephBlockPool))
			}
			cephBlockPoolCollector.collectMirroringStatus(cephBlockPools, ch)
			close(ch)
		}()

		for m := range ch {
			assert.Contains(t, m.Desc().String(), "status")
			metric.Reset()
			err := m.Write(&metric)
			assert.Nil(t, err)
			labels := metric.GetLabel()
			for _, label := range labels {
				if *label.Name == "name" {
					if *label.Value == objEnabled.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(1))
					} else if *label.Value == objDisabled.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(0))
					}
				} else if *label.Name == "namespace" {
					assert.Contains(t, cephBlockPoolCollector.AllowedNamespaces, *label.Value)
				} else if *label.Name == "rados_namespace" {
					assert.Contains(t, defaultRadosNamespace, *label.Value)
				}
			}
		}
	}

}

func TestCollectRadosNamespaceMirroringImageHealth(t *testing.T) {
	mockOpts.StopCh = make(chan struct{})
	defer close(mockOpts.StopCh)

	cephBlockPoolCollector := getMockCephBlockPoolCollector(t, mockOpts)

	objOk := mockCephBlockPoolRadosNamespace1.DeepCopy()
	objOk.Name = objOk.Name + "ok"
	objOk.Status = &cephv1.CephBlockPoolRadosNamespaceStatus{
		MirroringStatus: &cephv1.MirroringStatusSpec{
			MirroringStatus: cephv1.MirroringStatus{
				Summary: &cephv1.MirroringStatusSummarySpec{ImageHealth: "OK"},
			},
		},
	}

	objUnknown := mockCephBlockPoolRadosNamespace1.DeepCopy()
	objUnknown.Name = objUnknown.Name + "unknown"
	objUnknown.Status = &cephv1.CephBlockPoolRadosNamespaceStatus{

		MirroringStatus: &cephv1.MirroringStatusSpec{
			MirroringStatus: cephv1.MirroringStatus{
				Summary: &cephv1.MirroringStatusSummarySpec{ImageHealth: "UNKNOWN"},
			},
		},
	}

	objWarning := mockCephBlockPoolRadosNamespace1.DeepCopy()
	objWarning.Name = objWarning.Name + "warning"
	objWarning.Status = &cephv1.CephBlockPoolRadosNamespaceStatus{
		MirroringStatus: &cephv1.MirroringStatusSpec{
			MirroringStatus: cephv1.MirroringStatus{
				Summary: &cephv1.MirroringStatusSummarySpec{ImageHealth: "WARNING"},
			},
		},
	}

	objError := mockCephBlockPoolRadosNamespace1.DeepCopy()
	objError.Name = objError.Name + "error"
	objError.Status = &cephv1.CephBlockPoolRadosNamespaceStatus{
		MirroringStatus: &cephv1.MirroringStatusSpec{
			MirroringStatus: cephv1.MirroringStatus{
				Summary: &cephv1.MirroringStatusSummarySpec{ImageHealth: "ERROR"},
			},
		},
	}

	tests := Tests{
		{
			name: "Collect RadosNamespace mirroring image health metrics",
			args: args{
				objects: []runtime.Object{
					objOk,
					objUnknown,
					objWarning,
					objError,
				},
			},
		},
		{
			name: "Empty RadosNamespace",
			args: args{
				objects: []runtime.Object{},
			},
		},
	}

	for _, tt := range tests {
		ch := make(chan prometheus.Metric)
		metric := dto.Metric{}
		go func() {
			var radosNamespaces []*cephv1.CephBlockPoolRadosNamespace
			for _, obj := range tt.args.objects {
				radosNamespaces = append(radosNamespaces, obj.(*cephv1.CephBlockPoolRadosNamespace))
			}
			cephBlockPoolCollector.collectMirroringImageHealthRadosNamespace(radosNamespaces, ch)
			close(ch)
		}()

		for m := range ch {
			assert.Contains(t, m.Desc().String(), "image_health")
			metric.Reset()
			err := m.Write(&metric)
			assert.Nil(t, err)
			labels := metric.GetLabel()
			for _, label := range labels {
				if *label.Name == "rados_namespace" {
					if *label.Value == objOk.Spec.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(0))
					} else if *label.Value == objUnknown.Spec.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(1))
					} else if *label.Value == objWarning.Spec.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(2))
					} else if *label.Value == objError.Spec.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(3))
					}
				} else if *label.Name == "namespace" {
					assert.Contains(t, cephBlockPoolCollector.AllowedNamespaces, *label.Value)
				} else if *label.Name == "name" {
					assert.Equal(t, *label.Value, objOk.Spec.BlockPoolName)
				}
			}
		}
	}
}

func TestCollectRadosNamespaceMirroringStatus(t *testing.T) {
	mockOpts.StopCh = make(chan struct{})
	defer close(mockOpts.StopCh)

	cephBlockPoolCollector := getMockCephBlockPoolCollector(t, mockOpts)

	objPoolMode := mockCephBlockPoolRadosNamespace1.DeepCopy()
	objPoolMode.Name = objPoolMode.Name + "pool"
	objPoolMode.Spec = cephv1.CephBlockPoolRadosNamespaceSpec{
		Mirroring: &cephv1.RadosNamespaceMirroring{
			Mode: cephv1.RadosNamespaceMirroringModePool,
		},
	}

	objImageMode := mockCephBlockPoolRadosNamespace1.DeepCopy()
	objImageMode.Name = objImageMode.Name + "image"
	objImageMode.Spec = cephv1.CephBlockPoolRadosNamespaceSpec{
		Mirroring: &cephv1.RadosNamespaceMirroring{
			Mode: cephv1.RadosNamespaceMirroringModeImage,
		},
	}
	objDisabled := mockCephBlockPoolRadosNamespace1.DeepCopy()
	objDisabled.Name = objDisabled.Name + "disabled"
	objDisabled.Spec = cephv1.CephBlockPoolRadosNamespaceSpec{
		Mirroring: &cephv1.RadosNamespaceMirroring{
			Mode: "",
		},
	}
	tests := Tests{
		{
			name: "Collect RadosNamespace mirroring status",
			args: args{
				objects: []runtime.Object{
					objPoolMode,
					objImageMode,
					objDisabled,
				},
			},
		},
		{
			name: "Empty RadosNamespace",
			args: args{
				objects: []runtime.Object{},
			},
		},
	}

	for _, tt := range tests {
		ch := make(chan prometheus.Metric)
		metric := dto.Metric{}
		go func() {
			var radosNamespaces []*cephv1.CephBlockPoolRadosNamespace
			for _, obj := range tt.args.objects {
				radosNamespaces = append(radosNamespaces, obj.(*cephv1.CephBlockPoolRadosNamespace))
			}
			cephBlockPoolCollector.collectMirroringStatusRadosNamespace(radosNamespaces, ch)
			close(ch)
		}()

		for m := range ch {
			assert.Contains(t, m.Desc().String(), "status")
			metric.Reset()
			err := m.Write(&metric)
			assert.Nil(t, err)
			labels := metric.GetLabel()
			for _, label := range labels {
				if *label.Name == "rados_namespace" {
					if *label.Value == objPoolMode.Spec.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(1))
					} else if *label.Value == objImageMode.Spec.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(1))
					} else if *label.Value == objDisabled.Spec.Name {
						assert.Equal(t, *metric.Gauge.Value, float64(0))
					}
				} else if *label.Name == "namespace" {
					assert.Contains(t, cephBlockPoolCollector.AllowedNamespaces, *label.Value)
				} else if *label.Name == "rados_namespace" {
					assert.Contains(t, objImageMode.Spec.BlockPoolName, *label.Value)
				}
			}
		}
	}
}
