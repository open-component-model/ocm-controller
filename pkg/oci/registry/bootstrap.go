// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// defaultNamespace is the default namespace to deploy the registry
	defaultNamespace = "ocm-system"
	// defaultRegistreyImage is the default registry image to deploy
	defaultRegistreyImage = "registry:2"
	// defaultAppName is the default name of the registry deployment
	defaultAppName = "registry"
	// defaultRegistryPort is the default port of the registry service
	defaultRegistryPort = 5000
)

func main() {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	c, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not create client: %v", err)
		os.Exit(1)
	}
	ns := assignDefaultIfEmptyString(os.Getenv("NAMESPACE"), defaultNamespace)
	app := assignDefaultIfEmptyString(os.Getenv("APP_NAME"), defaultAppName)
	image := assignDefaultIfEmptyString(os.Getenv("REGISTRY_IMAGE"), defaultRegistreyImage)
	port, err := strconv.ParseInt(os.Getenv("REGISTRY_PORT"), 10, 32)
	if port == 0 || err != nil {
		port = defaultRegistryPort
	}
	// create registry deployment and service
	// TODO: add support for updating existing objects
	objs := registryObjects(ns, app, image, port)
	dep := &appsv1.Deployment{}
	if err := c.Get(ctx, client.ObjectKey{
		Namespace: ns,
		Name:      app}, dep); err != nil {
		if apierror.IsNotFound(err) {
			fmt.Fprintf(os.Stderr, "deployment %s/%s not found, creating resources", ns, app)
			applyObj(ctx, c, objs)
			os.Exit(0)
		} else {
			fmt.Fprintf(os.Stderr, "could not get deployment: %v", err)
			os.Exit(1)
		}
	}
	fmt.Fprintf(os.Stderr, "deployment %s/%s found, patching resources", ns, app)
	patchObj(ctx, c, objs)
	os.Exit(0)
}

// applyObj applies the given objects to the cluster
func applyObj(ctx context.Context, c client.Client, objs []client.Object) {
	for _, obj := range objs {
		if err := c.Create(ctx, obj, client.FieldOwner("ocm-controller")); err != nil {
			fmt.Fprintf(os.Stderr, "could not create object: %v", err)
			os.Exit(1)
		}
	}
}

// patchObj patches the given objects to the cluster. It patches only the spec
// field of the object.
func patchObj(ctx context.Context, c client.Client, objs []client.Object) {
	for _, obj := range objs {
		oldObj := obj.DeepCopyObject().(client.Object)
		if _, err := controllerutil.CreateOrPatch(ctx, c, oldObj, func() error {
			old, err := runtime.DefaultUnstructuredConverter.ToUnstructured(oldObj.DeepCopyObject())
			if err != nil {
				return err
			}
			new, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
			if err != nil {
				return err
			}
			spec, found, err := unstructured.NestedFieldCopy(new, "spec")
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("could not find spec field in object")
			}
			if err := unstructured.SetNestedField(old, spec, "spec"); err != nil {
				return err
			}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(old, oldObj); err != nil {
				return err
			}
			return nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, "could not patch object: %v", err)
			os.Exit(1)
		}
	}
}

// registryObjects returns the objects needed to deploy a registry
func registryObjects(namespace, name, image string, port int64) []client.Object {
	return []client.Object{
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:     name,
						Protocol: corev1.ProtocolTCP,
						Port:     int32(port),
						TargetPort: intstr.IntOrString{
							IntVal: int32(port),
						},
					},
				},
				Selector: map[string]string{
					"app": name,
				},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": name,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": name,
						},
					},
					Spec: corev1.PodSpec{
						EnableServiceLinks: new(bool),
						Containers: []corev1.Container{
							{
								Name: "registry",
								Env: []corev1.EnvVar{
									{
										Name:  "REGISTRY_STORAGE_DELETE_ENABLED",
										Value: "true",
									},
								},
								Image: image,
								LivenessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										TCPSocket: &corev1.TCPSocketAction{
											Port: intstr.IntOrString{
												IntVal: int32(port),
											},
										},
									},
									InitialDelaySeconds: 15,
									TimeoutSeconds:      1,
									PeriodSeconds:       20,
									SuccessThreshold:    1,
									FailureThreshold:    3,
								},
								ReadinessProbe: &corev1.Probe{
									ProbeHandler: corev1.ProbeHandler{
										TCPSocket: &corev1.TCPSocketAction{
											Port: intstr.IntOrString{
												IntVal: int32(port),
											},
										},
									},
									InitialDelaySeconds: 2,
									TimeoutSeconds:      1,
									PeriodSeconds:       5,
									SuccessThreshold:    1,
									FailureThreshold:    3,
								},
								SecurityContext: &corev1.SecurityContext{
									RunAsUser:                &[]int64{1000}[0],
									RunAsNonRoot:             &[]bool{true}[0],
									ReadOnlyRootFilesystem:   &[]bool{true}[0],
									AllowPrivilegeEscalation: new(bool),
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "registry",
										MountPath: "/var/lib/registry",
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
								Name: "registry",
							},
						},
					},
				},
			},
		},
	}
}

func assignDefaultIfEmptyString(s string, defaultVal string) string {
	if s == "" {
		s = defaultVal
	}
	return s
}
