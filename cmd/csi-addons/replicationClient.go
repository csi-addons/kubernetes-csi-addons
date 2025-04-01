/*
Copyright 2024 The Ceph-CSI Authors.

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

package main

import (
	"context"

	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type replicationClient struct {
	restClient *rest.RESTClient
}

func getVolumeReplicationClient() *replicationClient {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	scheme, err := replicationv1alpha1.SchemeBuilder.Build()
	if err != nil {
		panic(err.Error())
	}

	crdConfig := *config
	crdConfig.GroupVersion = &replicationv1alpha1.GroupVersion
	crdConfig.APIPath = "/apis"
	crdConfig.NegotiatedSerializer = serializer.NewCodecFactory(scheme)
	crdConfig.UserAgent = rest.DefaultKubernetesUserAgent()

	restClient, err := rest.UnversionedRESTClientFor(&crdConfig)
	if err != nil {
		panic(err)
	}

	return &replicationClient{restClient: restClient}
}

func (r *replicationClient) getVolumeGroupReplicationContent(ctx context.Context, name string, opts metav1.GetOptions) (*replicationv1alpha1.VolumeGroupReplicationContent, error) {
	result := replicationv1alpha1.VolumeGroupReplicationContent{}
	err := r.restClient.
		Get().
		Namespace("").
		Resource("volumegroupreplicationcontents").
		Name(name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(&result)

	return &result, err
}
