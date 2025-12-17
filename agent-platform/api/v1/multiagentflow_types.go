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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MultiAgentFlowSpec defines the desired state of MultiAgentFlow
type MultiAgentFlowSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	MCP         []MCPConfig        `json:"mcp,omitempty"`
	LLMS        []LLMConfig        `json:"llms,omitempty"`
	Logging     LoggingConfig      `json:"logging,omitempty"`
	Agents      []AgentConfig      `json:"agents,omitempty"`
	Connections []ConnectionConfig `json:"connections,omitempty"`
}

type MCPConfig struct {
	Name          string `json:"name,omitempty"`
	Type          string `json:"type,omitempty"`
	Endpoint      string `json:"endpoint,omitempty"`
	AuthSecretRef string `json:"authSecretRef,omitempty"`
}

type LLMConfig struct {
	Name            string            `json:"name,omitempty"`
	Provider        string            `json:"provider,omitempty"`
	Model           string            `json:"model,omitempty"`
	Endpoint        string            `json:"endpoint,omitempty"`
	Parameters      map[string]string `json:"parameters,omitempty"`
	ApiKeySecretRef string            `json:"apiKeySecretRef,omitempty"`
}

type LoggingConfig struct {
	// No Langfuse field; only Redis logging is supported
}

type ValidationRule struct {
	Type string `json:"type,omitempty"`
	Rule string `json:"rule,omitempty"`
}

type NodeConfig struct {
	ID          string `json:"id,omitempty"`
	Type        string `json:"type,omitempty"`
	LLMProvider string `json:"llmProvider,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	RagSource   string `json:"ragSource,omitempty"`
	Query       string `json:"query,omitempty"`
}

type EdgeConfig struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

type AgentConfig struct {
	Name       string           `json:"name,omitempty"`
	LLMS       []LLMConfig      `json:"llms,omitempty"`
	Validation []ValidationRule `json:"validation,omitempty"`
	Nodes      []NodeConfig     `json:"nodes,omitempty"`
	Edges      []EdgeConfig     `json:"edges,omitempty"`
	MCP        []MCPConfig      `json:"mcp,omitempty"`
}

type ConnectionConfig struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

// MultiAgentFlowStatus defines the observed state of MultiAgentFlow.
type MultiAgentFlowStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the MultiAgentFlow resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MultiAgentFlow is the Schema for the multiagentflows API
type MultiAgentFlow struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of MultiAgentFlow
	// +required
	Spec MultiAgentFlowSpec `json:"spec"`

	// status defines the observed state of MultiAgentFlow
	// +optional
	Status MultiAgentFlowStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// MultiAgentFlowList contains a list of MultiAgentFlow
type MultiAgentFlowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []MultiAgentFlow `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MultiAgentFlow{}, &MultiAgentFlowList{})
}
