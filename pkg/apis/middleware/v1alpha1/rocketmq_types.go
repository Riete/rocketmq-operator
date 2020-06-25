package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RoleName string

const (
	MasterRole      RoleName = "master"
	SlaveRole       RoleName = "slave"
	NamesrvRole     RoleName = "namesrv"
	ConsoleRole     RoleName = "console"
	ConsoleImage    string   = "riet/rocketmq-console-ng:v1.0.0"
	BrokerBaseImage string   = "riet/rocketmq:4.4.0-broker"
	NameSrvImage    string   = "riet/rocketmq:4.4.0-namesrv"
	ConsolePort     int32    = 8080
	NamesrvPort     int32    = 9876
	BrokerPort      int32    = 10911
)

type NameSrv struct {
	Num       int32                       `json:"num"`
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	Xmx       string                      `json:"xmx"`
	Xmn       string                      `json:"xmn"`
}

type Broker struct {
	Num                 int32                       `json:"num"`
	Resources           corev1.ResourceRequirements `json:"resources,omitempty"`
	Xmx                 string                      `json:"xmx"`
	Xmn                 string                      `json:"xmn"`
	MaxDirectMemorySize string                      `json:"maxDirectMemorySize"`
	StorageClass        string                      `json:"storageClass,omitempty"`
	StorageSize         string                      `json:"storageSize,omitempty"`
	Mode                string                      `json:"mode"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RocketMQSpec defines the desired state of RocketMQ
type RocketMQSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	NameSrv     NameSrv `json:"nameSrv"`
	MasterSlave Broker  `json:"masterSlave"`
	Console     bool    `json:"console,omitempty"`
}

// RocketMQStatus defines the observed state of RocketMQ
type RocketMQStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RocketMQ is the Schema for the rocketmqs API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=rocketmqs,scope=Namespaced
type RocketMQ struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RocketMQSpec   `json:"spec,omitempty"`
	Status RocketMQStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RocketMQList contains a list of RocketMQ
type RocketMQList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RocketMQ `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RocketMQ{}, &RocketMQList{})
}
