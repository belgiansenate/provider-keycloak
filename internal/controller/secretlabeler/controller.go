/*
Copyright 2024 Belgian Senate.

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

// Package secretlabeler provides a controller that patches connection secrets
// with labels/annotations specified on managed resources.
package secretlabeler

import (
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
)

const (
	// AnnotationConnectionSecretLabels is the annotation key for specifying
	// labels to add to connection secrets. Value should be a JSON object.
	// Example: {"app.kubernetes.io/part-of":"argocd"}
	AnnotationConnectionSecretLabels = "keycloak.crossplane.io/connection-secret-labels"

	// AnnotationConnectionSecretAnnotations is the annotation key for specifying
	// annotations to add to connection secrets. Value should be a JSON object.
	AnnotationConnectionSecretAnnotations = "keycloak.crossplane.io/connection-secret-annotations"

	// AnnotationSecretLabelsApplied tracks whether labels have been applied
	AnnotationSecretLabelsApplied = "keycloak.crossplane.io/labels-applied"
)

// SecretLabelerReconciler reconciles managed resources and patches their
// connection secrets with specified labels/annotations.
type SecretLabelerReconciler struct {
	client.Client
	GVK schema.GroupVersionKind
}

// Reconcile patches the connection secret of a managed resource with labels
// specified in the managed resource's annotations.
func (r *SecretLabelerReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	// Get the managed resource
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(r.GVK)
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	// Get annotations
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return reconcile.Result{}, nil
	}

	// Check if labels/annotations are specified
	labelsJSON, hasLabels := annotations[AnnotationConnectionSecretLabels]
	annotationsJSON, hasAnnotations := annotations[AnnotationConnectionSecretAnnotations]

	if !hasLabels && !hasAnnotations {
		return reconcile.Result{}, nil
	}

	// Get writeConnectionSecretToRef from spec
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		return reconcile.Result{}, nil
	}

	secretRef, found, err := unstructured.NestedMap(spec, "writeConnectionSecretToRef")
	if err != nil || !found {
		return reconcile.Result{}, nil
	}

	secretName, _, _ := unstructured.NestedString(secretRef, "name")
	secretNamespace, _, _ := unstructured.NestedString(secretRef, "namespace")

	if secretName == "" || secretNamespace == "" {
		return reconcile.Result{}, nil
	}

	// Parse labels
	labels := make(map[string]string)
	if hasLabels && labelsJSON != "" {
		if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
			logger.Error(err, "cannot parse connection secret labels annotation")
			return reconcile.Result{}, nil // Don't requeue for parse errors
		}
	}

	// Parse annotations
	secretAnnotations := make(map[string]string)
	if hasAnnotations && annotationsJSON != "" {
		if err := json.Unmarshal([]byte(annotationsJSON), &secretAnnotations); err != nil {
			logger.Error(err, "cannot parse connection secret annotations annotation")
			return reconcile.Result{}, nil // Don't requeue for parse errors
		}
	}

	if len(labels) == 0 && len(secretAnnotations) == 0 {
		return reconcile.Result{}, nil
	}

	// Get the secret
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: secretNamespace,
	}, secret); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return reconcile.Result{}, errors.Wrap(err, "cannot get connection secret")
		}
		// Secret doesn't exist yet, requeue
		return reconcile.Result{RequeueAfter: xpv1.LongWait}, nil
	}

	// Check if update is needed
	needsUpdate := false

	// Merge labels
	if len(labels) > 0 {
		if secret.Labels == nil {
			secret.Labels = make(map[string]string)
		}
		for k, v := range labels {
			if secret.Labels[k] != v {
				secret.Labels[k] = v
				needsUpdate = true
			}
		}
	}

	// Merge annotations
	if len(secretAnnotations) > 0 {
		if secret.Annotations == nil {
			secret.Annotations = make(map[string]string)
		}
		for k, v := range secretAnnotations {
			if secret.Annotations[k] != v {
				secret.Annotations[k] = v
				needsUpdate = true
			}
		}
	}

	// Only update if there were changes
	if !needsUpdate {
		return reconcile.Result{}, nil
	}

	logger.Info("patching connection secret with labels",
		"secret", secretName,
		"namespace", secretNamespace,
		"labels", labels)

	if err := r.Update(ctx, secret); err != nil {
		return reconcile.Result{}, errors.Wrap(err, "cannot update connection secret")
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager for a specific GVK.
func (r *SecretLabelerReconciler) SetupWithManager(mgr ctrl.Manager, gvk schema.GroupVersionKind) error {
	r.GVK = gvk

	// Create an unstructured object to watch
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)

	return ctrl.NewControllerManagedBy(mgr).
		Named("secretlabeler-" + gvk.Kind).
		For(obj).
		// Also watch secrets to handle cases where the secret is created after the MR
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
			// When a secret changes, find MRs that reference it
			// This is a simplified approach - in production you might want to index this
			return nil // For now, rely on MR reconciliation
		})).
		Complete(r)
}

// SetupSecretLabelers sets up secret labeler controllers for all relevant GVKs.
func SetupSecretLabelers(mgr ctrl.Manager) error {
	// GVKs that support connection secrets and might need labels
	gvks := []schema.GroupVersionKind{
		{Group: "openidclient.keycloak.crossplane.io", Version: "v1alpha1", Kind: "Client"},
		// Add other GVKs as needed
	}

	for _, gvk := range gvks {
		r := &SecretLabelerReconciler{Client: mgr.GetClient()}
		if err := r.SetupWithManager(mgr, gvk); err != nil {
			return errors.Wrapf(err, "cannot setup secret labeler for %s", gvk.String())
		}
	}

	return nil
}
