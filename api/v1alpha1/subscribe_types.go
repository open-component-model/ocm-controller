package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Payload defines what type of payload to send. Templated or none templated.
type Payload struct {
	// Template can be used to fine-tune the payload with additional information from the
	// context object.
	Template string `json:"template,omitempty"`
	// Raw can be used for un-altered plain data.
	Raw apiextensionsv1.JSON `json:"raw,omitempty"`
}

// SubscribeSpec defines subscription values. Multiple events can be subscribed to.
type SubscribeSpec struct {
	// Payload is a Go Template constructed from the objects that it belongs to.
	// +required
	Payload Payload `json:"payload"`
	// URL is the location on which the payload will be delivered. There is no retry!
	// +required
	URL string `json:"url"`
	// Condition is the condition that the object publishes and the user subscribes to.
	// +required
	Condition string `json:"condition"`
	// Status is the outcome value of the event. Usually `success` or `failure` or `deployed`.
	// +required
	Status metav1.ConditionStatus `json:"status"`
}

type Request struct {
	Payload     string
	Destination string
}

// Subscribable defines an object that can send subscription events.
// +k8s:deepcopy-gen=false
type Subscribable interface {
	Payloads() []Request
}
