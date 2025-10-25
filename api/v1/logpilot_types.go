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

// LogPilotSpec defines the desired state of LogPilot
type LogPilotSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// foo is an example field of LogPilot. Edit logpilot_types.go to remove/update

	// Loki URL
	LokiURL string `json:"lokiURL"`
	// Loki PromQL
	LokiPromQL string `json:"lokiPromQL"`
	// LLM Endpoint
	LLMEndpoint string `json:"llmEndpoint"`
	// LLM Token
	LLMToken string `json:"llmToken"`
	// LLM Model
	LLMModel string `json:"llmModel"`
	// Feishu Webhook
	FeishuWebhook string `json:"feishuWebhook"`
}

// LogPilotStatus defines the observed state of LogPilot.
type LogPilotStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	PreTimeStamp string `json:"preTimestamp"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// LogPilot is the Schema for the logpilots API
type LogPilot struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of LogPilot
	// +required
	Spec LogPilotSpec `json:"spec"`

	// status defines the observed state of LogPilot
	// +optional
	Status LogPilotStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// LogPilotList contains a list of LogPilot
type LogPilotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogPilot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LogPilot{}, &LogPilotList{})
}
