/*


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

import (
	"context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	codev1alpha1 "github.com/Ressetkk/vsc-kube/api/v1alpha1"
)

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=code.resset.xyz,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=code.resset.xyz,resources=workspaces/status,verbs=get;update;patch

func (r *WorkspaceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("workspace", req.NamespacedName)

	workspace := &codev1alpha1.Workspace{}
	err := r.Get(ctx, req.NamespacedName, workspace)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Workspace resource not found. Ignoring...")
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "Failed to get Workspace element.")
		return ctrl.Result{}, err
	}

	workspaceForDeletion := workspace.GetDeletionTimestamp() != nil
	if workspaceForDeletion {
		// TODO (@Ressetkk): Deletion finalizers and garbage collection
		r.Log.Info("Workspace deleted.")
		return ctrl.Result{}, nil
	}

	found := &v1.Pod{}
	err = r.Get(ctx, types.NamespacedName{Name: workspace.Name, Namespace: workspace.Namespace}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			newPod := r.podForWorkspace(workspace)
			r.Log.Info("Create new pod for workspace.", "name", newPod.Name, "namespace", newPod.Namespace)
			err = r.Create(ctx, newPod)
			if err != nil {
				r.Log.Error(err, "Could not create pod.", "name", newPod.Name, "namespace", newPod.Namespace)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		r.Log.Error(err, "Could not get pod.")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&codev1alpha1.Workspace{}).
		Complete(r)
}

func (r *WorkspaceReconciler) podForWorkspace(ws *codev1alpha1.Workspace) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ws.Name,
			Namespace: ws.Namespace,
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{
				{
					// TODO (@Ressetkk): use own implementation of cloning repositories
					// 	support for private repositories - personal access token
					Name:       "clonerefs",
					Image:      "alpine/git:latest",
					WorkingDir: "/workspace",
					Args: []string{
						"clone",
						ws.Spec.Repo.URL,
						ws.Spec.Repo.Name,
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "workdir",
							MountPath: "/workspace",
						},
					},
				},
			},
			Containers: []v1.Container{
				{
					Name:      "workspace",
					Image:     ws.Spec.Image,
					Resources: ws.Spec.Resources,
					Args: []string{
						"--auth", "none",
						"/workspace/" + ws.Spec.Repo.Name,
					},
					Ports: []v1.ContainerPort{
						{
							ContainerPort: 8080,
							Name:          "vsc-port",
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "workdir",
							MountPath: "/workspace",
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name:         "workdir",
					VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}},
				},
			},
		},
	}
	ctrl.SetControllerReference(ws, pod, r.Scheme)
	return pod
}
