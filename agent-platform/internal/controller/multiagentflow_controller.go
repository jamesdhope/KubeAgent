/*
Copyright 2025.

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
	// ...existing code...
	"context"
	"encoding/json"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	"github.com/go-redis/redis/v8"
	agentplatformv1 "github.com/jamesdhope/agent-platform/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// marshalAgentSpec serializes the agent spec to a JSON string for pod env

func marshalAgentSpec(agent agentplatformv1.AgentConfig) string {
	b, err := json.Marshal(agent)
	if err != nil {
		return "{}"
	}
	return string(b)
}

const PythonServerAppLabel = "python-server"

// MultiAgentFlowReconciler reconciles a MultiAgentFlow object
type MultiAgentFlowReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// buildAgentDeps builds a dependency graph for agents from the flow's connections
func buildAgentDeps(connections []agentplatformv1.ConnectionConfig) map[string][]string {
	agentDeps := make(map[string][]string)
	for _, edge := range connections {
		fromParts := strings.Split(edge.From, ".")
		toParts := strings.Split(edge.To, ".")
		if len(toParts) > 0 && len(fromParts) > 0 {
			agentTo := toParts[0]
			agentFrom := fromParts[0]
			agentDeps[agentTo] = append(agentDeps[agentTo], agentFrom)
		}
	}
	return agentDeps
}

// createAgentPods creates agent pods and writes their dependencies to Redis
func createAgentPods(ctx context.Context, c client.Client, flow *agentplatformv1.MultiAgentFlow, agentDeps map[string][]string, redisSvcName string, rdb *redis.Client, log logr.Logger) error {
	for _, agent := range flow.Spec.Agents {
		agentName := agent.Name
		// Check agent completion in Redis
		completeKey := flow.Name + ":" + agentName + ":complete"
		completeExists, err := rdb.Exists(ctx, completeKey).Result()
		if err != nil {
			log.Error(err, "error checking agent completion in Redis", "agent", agentName)
			continue // skip pod creation if Redis error
		}
		if completeExists > 0 {
			log.Info("Agent already complete, skipping pod creation", "agent", agentName)
			continue
		}
		agentLabels := map[string]string{"app": "agent-worker", "multiagentflow": flow.Name, "agent": agentName}
		dependencies := agentDeps[agentName]
		depKey := flow.Name + ":dependencies:" + agentName
		depVal, _ := json.Marshal(dependencies)
		rdb.Set(ctx, depKey, depVal, 0)
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      flow.Name + "-" + agentName,
				Namespace: flow.Namespace,
				Labels:    agentLabels,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "agent-worker",
				Containers: []corev1.Container{{
					Name:            "agent-worker",
					Image:           "agent-platform-python-server:latest",
					ImagePullPolicy: corev1.PullNever,
					Env: []corev1.EnvVar{{Name: "AGENT_NAME", Value: agentName},
						{Name: "AGENT_SPEC", Value: marshalAgentSpec(agent)},
						{Name: "REDIS_HOST", Value: redisSvcName},
						{Name: "REDIS_PORT", Value: "6379"},
						{Name: "GRAPH_ID", Value: flow.Name},
					},
					Ports: []corev1.ContainerPort{{ContainerPort: 5000}},
				}},
			},
		}
		var existingPod corev1.Pod
		err = c.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: pod.Name}, &existingPod)
		if apierrors.IsNotFound(err) {
			err = c.Create(ctx, pod)
			if err != nil {
				log.Error(err, "unable to create agent pod", "agent", agentName)
				return err
			}
			log.Info("Created agent pod", "name", pod.Name)
		}
	}
	return nil
}

// checkAllAgentsDone checks Redis for agent output keys to determine if all agents are done
func checkAllAgentsDone(ctx context.Context, flow *agentplatformv1.MultiAgentFlow, rdb *redis.Client) (bool, error) {
	for _, agent := range flow.Spec.Agents {
		key := flow.Name + ":" + agent.Name + ":output"
		exists, err := rdb.Exists(ctx, key).Result()
		if err != nil || exists == 0 {
			return false, err
		}
	}
	return true, nil
}

// cleanupAgentPods deletes all agent pods after completion
func cleanupAgentPods(ctx context.Context, c client.Client, flow *agentplatformv1.MultiAgentFlow, log logr.Logger) {
	for _, agent := range flow.Spec.Agents {
		podName := flow.Name + "-" + agent.Name
		err := c.Delete(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: flow.Namespace, Name: podName}})
		if err != nil && !apierrors.IsNotFound(err) {
			log.Error(err, "unable to delete agent pod", "pod", podName)
		}
	}
}

// +kubebuilder:rbac:groups=agentplatform.example.com,resources=multiagentflows,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=agentplatform.example.com,resources=multiagentflows/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=agentplatform.example.com,resources=multiagentflows/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.4/pkg/reconcile
func (r *MultiAgentFlowReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	var flow agentplatformv1.MultiAgentFlow
	if err := r.Get(ctx, req.NamespacedName, &flow); err != nil {
		log.Error(err, "unable to fetch MultiAgentFlow")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// If already completed, do not retrigger graph
	for _, cond := range flow.Status.Conditions {
		if cond.Type == "Completed" && cond.Status == metav1.ConditionTrue {
			log.Info("MultiAgentFlow already completed, skipping orchestration.")
			return ctrl.Result{}, nil
		}
	}
	log.Info("Reconciling MultiAgentFlow", "name", req.NamespacedName)

	// 1. If already completed, do not retrigger graph
	for _, cond := range flow.Status.Conditions {
		if cond.Type == "Completed" && cond.Status == metav1.ConditionTrue {
			log.Info("MultiAgentFlow already completed, skipping orchestration.")
			return ctrl.Result{}, nil
		}
	}
	log.Info("Reconciling MultiAgentFlow", "name", req.NamespacedName)

	// 2. Ensure Redis Service and Deployment exist
	if err := ensureRedis(ctx, r.Client, flow.Namespace, log); err != nil {
		return ctrl.Result{}, err
	}

	// 3. Build agent dependency graph from connections
	agentDeps := buildAgentDeps(flow.Spec.Connections)

	// 4. Create agent pods and write dependencies to Redis
	redisAddr := "redis.default.svc.cluster.local:6379"
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	if err := createAgentPods(ctx, r.Client, &flow, agentDeps, "redis", rdb, log); err != nil {
		return ctrl.Result{}, err
	}

	// 5. Monitor agent pod status and update CR status when all agents complete
	allDone, err := checkAllAgentsDone(ctx, &flow, rdb)
	if err != nil {
		log.Error(err, "error checking agent completion")
		return ctrl.Result{}, err
	}
	if allDone {
		// Update CR status to completed
		flow.Status.Conditions = []metav1.Condition{{
			Type:               "Completed",
			Status:             metav1.ConditionTrue,
			Reason:             "AllAgentsSucceeded",
			Message:            "All agent outputs present in Redis.",
			LastTransitionTime: metav1.Now(),
		}}
		// Retry status update on conflict
		maxRetries := 5
		for i := 0; i < maxRetries; i++ {
			err = r.Status().Update(ctx, &flow)
			if err == nil {
				// Cleanup agent pods
				cleanupAgentPods(ctx, r.Client, &flow, log)
				return ctrl.Result{}, nil
			}
			if apierrors.IsConflict(err) {
				log.Info("Conflict updating status, retrying", "attempt", i+1)
				// Re-fetch latest resource
				var latestFlow agentplatformv1.MultiAgentFlow
				errGet := r.Get(ctx, req.NamespacedName, &latestFlow)
				if errGet != nil {
					log.Error(errGet, "unable to fetch latest MultiAgentFlow for status update retry")
					break
				}
				flow = latestFlow
				flow.Status.Conditions = []metav1.Condition{{
					Type:               "Completed",
					Status:             metav1.ConditionTrue,
					Reason:             "AllAgentsSucceeded",
					Message:            "All agent outputs present in Redis.",
					LastTransitionTime: metav1.Now(),
				}}
				continue
			}
			log.Error(err, "unable to update MultiAgentFlow status after completion")
			break
		}
		return ctrl.Result{}, err
	}

	// 6. Update status (example: Available)
	flow.Status.Conditions = []metav1.Condition{{
		Type:               "Available",
		Status:             metav1.ConditionTrue,
		Reason:             "PythonServerReady",
		Message:            "Python server deployment is available",
		LastTransitionTime: metav1.Now(),
	}}
	err = r.Status().Update(ctx, &flow)
	if err != nil {
		log.Error(err, "unable to update MultiAgentFlow status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// ensureDeployment creates the Deployment if it does not exist
func ensureDeployment(ctx context.Context, r client.Client, deployment *appsv1.Deployment, log logr.Logger) error {
	var existing appsv1.Deployment
	err := r.Get(ctx, client.ObjectKey{Namespace: deployment.Namespace, Name: deployment.Name}, &existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, deployment); err != nil {
				log.Error(err, "unable to create Python server Deployment")
				return err
			}
			log.Info("Created Python server Deployment", "name", deployment.Name)
		}
		return err
	}
	return nil
}

// ensureRedis ensures the Redis Service and Deployment exist in the namespace
func ensureRedis(ctx context.Context, c client.Client, namespace string, log logr.Logger) error {
	redisLabels := map[string]string{"app": "redis"}
	redisSvcName := "redis"
	redisDeploymentName := "redis"
	redisService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      redisSvcName,
			Namespace: namespace,
			Labels:    redisLabels,
		},
		Spec: corev1.ServiceSpec{
			Selector: redisLabels,
			Ports: []corev1.ServicePort{{
				Protocol:   corev1.ProtocolTCP,
				Port:       6379,
				TargetPort: intstr.FromInt(6379),
			}},
		},
	}
	if err := ensureService(ctx, c, redisService, log); err != nil {
		return err
	}
	redisDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      redisDeploymentName,
			Namespace: namespace,
			Labels:    redisLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: redisLabels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: redisLabels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "redis",
						Image: "redis:7-alpine",
						Ports: []corev1.ContainerPort{{ContainerPort: 6379}},
					}},
				},
			},
		},
	}
	if err := ensureDeployment(ctx, c, redisDeployment, log); err != nil {
		return err
	}
	return nil
}
func ensureService(ctx context.Context, c client.Client, svc *corev1.Service, log logr.Logger) error {
	var existing corev1.Service
	err := c.Get(ctx, client.ObjectKey{Namespace: svc.Namespace, Name: svc.Name}, &existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := c.Create(ctx, svc); err != nil {
				log.Error(err, "unable to create Service", "name", svc.Name)
				return err
			}
			log.Info("Created Service", "name", svc.Name)
		}
		return err
	}
	return nil
}

// desiredPythonServerDeployment returns a Deployment object for the Python server
func desiredPythonServerDeployment(flow *agentplatformv1.MultiAgentFlow) *appsv1.Deployment {
	labels := map[string]string{"app": PythonServerAppLabel, "multiagentflow": flow.Name}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      flow.Name + "-python-server",
			Namespace: flow.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "python-server",
						Image:           "agent-platform-python-server:latest",
						ImagePullPolicy: corev1.PullNever,
						Ports:           []corev1.ContainerPort{{ContainerPort: 5000}},
					}},
				},
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *MultiAgentFlowReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentplatformv1.MultiAgentFlow{}).
		Named("multiagentflow").
		Complete(r)
}
