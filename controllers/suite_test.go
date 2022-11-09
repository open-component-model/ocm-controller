/*
Copyright 2022.

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

package controllers

//import (
//	"fmt"
//	"path/filepath"
//	"testing"
//
//	. "github.com/onsi/gomega"
//
//	deliveryv1alpha1 "github.com/open-component-model/ocm-controller/api/v1alpha1"
//	"k8s.io/client-go/rest"
//	"k8s.io/kubectl/pkg/scheme"
//	"sigs.k8s.io/controller-runtime/pkg/client"
//	"sigs.k8s.io/controller-runtime/pkg/envtest"
//	//+kubebuilder:scaffold:imports
//)
//
//var (
//	cfg       *rest.Config
//	k8sClient client.Client
//	testEnv   *envtest.Environment
//)
//
//func setup() {
//	testEnv = &envtest.Environment{
//		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
//		ErrorIfCRDPathMissing: true,
//	}
//
//	// cfg is defined in this file globally.
//	var err error
//	cfg, err = testEnv.Start()
//	if err != nil {
//		panic(fmt.Errorf("failed to start test environment: %w", err))
//	}
//
//	if err = deliveryv1alpha1.AddToScheme(scheme.Scheme); err != nil {
//		panic(fmt.Errorf("failed to add scheme: %w", err))
//	}
//
//	err = deliveryv1alpha1.AddToScheme(scheme.Scheme)
//	Expect(err).NotTo(HaveOccurred())
//
//	//+kubebuilder:scaffold:scheme
//	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
//	if err != nil {
//		panic(fmt.Errorf("failed to create client: %w", err))
//	}
//}
//
//func teardown() {
//	if err := testEnv.Stop(); err != nil {
//		panic(fmt.Errorf("failed to stop test environment: %w", err))
//	}
//}
//
//func TestMain(m *testing.M) {
//	setup()
//	defer teardown()
//	m.Run()
//}
