/*
Copyright 2023.

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

package controller

import (
	"context"
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	argowfv1alpha1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"

	workflowv1 "github.com/NexClipper/wfwatch/api/v1"
)

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	Dynamic dynamic.Interface
	Scheme  *runtime.Scheme
}

//+kubebuilder:rbac:groups=workflow.nexclipper.io,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=workflow.nexclipper.io,resources=services/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=workflow.nexclipper.io,resources=services/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Service object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Start Service Reconcile")

	var service = &workflowv1.Service{}
	if err := r.Get(ctx, req.NamespacedName, service); err != nil {
		log.Error(err, "unable to fetch Service")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// b, err := json.Marshal(service)
	// if err != nil {
	// 	log.Error(err, "failed to marshal Service")
	// 	return ctrl.Result{}, err
	// }

	// log.Info(string(b))
	argowf, err := r.argoWorkflowForService(service)
	if err != nil {
		log.Error(err, "failed to create argo workflow from Service")
		return ctrl.Result{}, err
	}

	log.Info("Creating a new Workflow", "Workflow.Namespace", argowf.Namespace, "Workflow.Name", argowf.Name)
	b, _ := json.Marshal(argowf)
	m := make(map[string]interface{})
	json.Unmarshal(b, &m)
	uns := &unstructured.Unstructured{Object: m}
	// uns.SetUnstructuredContent(m)
	// wwf, err := r.Dynamic.Resource(schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "workflows"}).Namespace("argo").Create(ctx, uns, metav1.CreateOptions{})
	// if err != nil {
	// 	log.Error(err, "Failed to create new Workflow", "Workflow.Namespace", argowf.Namespace, "Workflow.Name", argowf.Name)
	// 	return ctrl.Result{}, err
	// }
	// generatedName := wwf.GetName()
	if err := r.Create(ctx, uns); err != nil {
		log.Error(err, "Failed to create new Workflow", "Workflow.Namespace", argowf.Namespace, "Workflow.Name", argowf.Name)
		return ctrl.Result{}, err
	}
	generatedName := argowf.GetName()
	log.Info("Created Workflow", "generatedName", generatedName)
	// Deployment created successfully - return and requeue
	// return ctrl.Result{Requeue: true}, nil

	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) argoWorkflowForService(s *workflowv1.Service) (*argowfv1alpha1.Workflow, error) {
	wf := &argowfv1alpha1.Workflow{}
	b, err := yaml.ToJSON([]byte(s.Spec.Workflow.Source))
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, wf); err != nil {
		return nil, err
	}
	wf.Namespace = s.Spec.Workflow.Namespace

	// Set Memcached instance as the owner and controller
	// if err := ctrl.SetControllerReference(s, wf, r.Scheme); err != nil {
	// 	return nil, err
	// }
	// set the owner references for workflow
	ownerReferences := wf.GetOwnerReferences()
	trueVar := true
	newRef := metav1.OwnerReference{
		Kind:       "Service",
		APIVersion: "v1",
		Name:       s.GetName(),
		UID:        s.GetUID(),
		Controller: &trueVar,
	}
	ownerReferences = append(ownerReferences, newRef)
	wf.SetOwnerReferences(ownerReferences)
	return wf, nil
}

func labelsForWfwatch() map[string]string {
	return map[string]string{"app": "wfwatch"}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&workflowv1.Service{}).
		Complete(r)
}

var _ = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "workflows",
}
