/*
Copyright 2023 The Kubernetes-CSI-Addons Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package envtest

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"github.com/csi-addons/spec/lib/go/identity"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	volumegroupv1 "github.com/csi-addons/kubernetes-csi-addons/apis/volumegroup.storage/v1"
	vg_controller "github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage"
	"github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/envtest/utils"
	vgc_controller "github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/volumegroupcontent"
	"github.com/csi-addons/kubernetes-csi-addons/internal/client/fake"
	conn "github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	addonsUtils "github.com/csi-addons/kubernetes-csi-addons/internal/util"
	"github.com/csi-addons/kubernetes-csi-addons/tests/mock_grpc_server"
	//+kubebuilder:scaffold:imports
)

var (
	cfg          *rest.Config
	k8sClient    client.Client
	testEnv      *envtest.Environment
	cancel       context.CancelFunc
	ctx          context.Context
	server       *mock_grpc_server.MockServer
	addonsConfig = addonsUtils.NewConfig()
	ctrlOptions  = controller.Options{
		MaxConcurrentReconciles: addonsConfig.MaxConcurrentReconciles,
	}
	defaultTimeout = time.Minute * 3
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = volumegroupv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	server, err = mock_grpc_server.CreateMockServer()
	Expect(err).ToNot(HaveOccurred())
	addr := server.Address()
	csiConn, err := fake.New(addr, DriverName)
	vgCapability := identity.Capability{
		Type: &identity.Capability_VolumeGroup_{
			VolumeGroup: &identity.Capability_VolumeGroup{
				Type: identity.Capability_VolumeGroup_VOLUME_GROUP,
			},
		},
	}
	vgModifyCapability := identity.Capability{
		Type: &identity.Capability_VolumeGroup_{
			VolumeGroup: &identity.Capability_VolumeGroup{
				Type: identity.Capability_VolumeGroup_MODIFY_VOLUME_GROUP,
			},
		},
	}

	csiConn.Capabilities = append(csiConn.Capabilities, &vgCapability)
	csiConn.Capabilities = append(csiConn.Capabilities, &vgModifyCapability)
	connPool := conn.NewConnectionPool()
	connPool.Put(DriverName, csiConn)
	Expect(err).ToNot(HaveOccurred())

	err = (&vg_controller.VolumeGroupReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Log:      ctrl.Log.WithName("controllers").WithName("VolumeGroup"),
		Connpool: connPool,
		Timeout:  defaultTimeout,
	}).SetupWithManager(mgr, ctrlOptions)
	Expect(err).ToNot(HaveOccurred())

	err = (&vgc_controller.VolumeGroupContentReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Log:      ctrl.Log.WithName("VolumeGroupContentController"),
		Connpool: connPool,
		Timeout:  defaultTimeout,
	}).SetupWithManager(mgr, ctrlOptions)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = mgr.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

}, 60)

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	server.Stop()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func createNonVolumeK8SResources() error {
	err := utils.CreateResourceObject(Secret, k8sClient)
	if err != nil {
		return err
	}
	return utils.CreateResourceObject(StorageClass, k8sClient)
}

func createVolumeGroupObjects(deletionPolicy volumegroupv1.VolumeGroupDeletionPolicy) error {
	err := utils.CreateResourceObject(VGClass, k8sClient)
	if err != nil {
		return err
	}

	vgclass := &volumegroupv1.VolumeGroupClass{}
	err = utils.GetNamespacedResourceObject(VGClassName, Namespace, vgclass, k8sClient)
	if err != nil {
		return err
	}
	vgclass.VolumeGroupDeletionPolicy = &deletionPolicy
	err = k8sClient.Update(context.TODO(), vgclass)
	if err != nil {
		return err
	}

	err = utils.CreateResourceObject(VG, k8sClient)
	return err
}

func createVolumeObjects() error {
	if err := utils.CreateResourceObject(PV, k8sClient); err != nil {
		return err
	}
	if err := utils.CreateResourceObject(PVC, k8sClient); err != nil {
		return err
	}
	pvc := &corev1.PersistentVolumeClaim{}
	if err := utils.GetNamespacedResourceObject(PVCName, Namespace, pvc, k8sClient); err != nil {
		return err
	}
	pvc.Status.Phase = corev1.ClaimBound
	err := k8sClient.Status().Update(context.TODO(), pvc)
	return err
}

func cleanTestNamespace() error {
	err := cleanVolumeGroupObjects()
	if err != nil {
		return err
	}
	err = cleanVolumeObjects()
	if err != nil {
		return err
	}
	err = k8sClient.DeleteAllOf(context.Background(), &corev1.Secret{}, client.InNamespace(Namespace))
	if err != nil {
		return err
	}
	err = k8sClient.DeleteAllOf(context.Background(), &storagev1.StorageClass{})
	return err
}

func cleanVolumeGroupObjects() error {
	err := k8sClient.DeleteAllOf(context.Background(), &volumegroupv1.VolumeGroup{}, client.InNamespace(Namespace))
	if err != nil {
		return err
	}
	err = k8sClient.DeleteAllOf(context.Background(), &volumegroupv1.VolumeGroupContent{}, client.InNamespace(Namespace))
	if err != nil {
		return err
	}
	err = k8sClient.DeleteAllOf(context.Background(), &volumegroupv1.VolumeGroupClass{}, client.InNamespace(Namespace))
	return err
}

func cleanVolumeObjects() error {
	pvc := &corev1.PersistentVolumeClaim{}
	if err := utils.RemoveResourceObjectFinalizers(PVCName, Namespace, pvc, k8sClient); err != nil {
		return err
	}
	pv := &corev1.PersistentVolume{}
	if err := utils.RemoveResourceObjectFinalizers(PVName, Namespace, pv, k8sClient); err != nil {
		return err
	}
	err := k8sClient.DeleteAllOf(context.Background(), &corev1.PersistentVolumeClaim{}, client.InNamespace(Namespace))
	if err != nil {
		return err
	}
	err = k8sClient.DeleteAllOf(context.Background(), &corev1.PersistentVolume{})
	return err
}
