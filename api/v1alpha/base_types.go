package v1alpha

// DoltClusterRef is a reference to a DoltDB object.
type DoltClusterRef struct {
	// ObjectReference is a reference to a object.
	ObjectReference `json:",inline"`
	// WaitForIt indicates whether the controller using this reference should wait for DoltDB to be ready.
	// +optional
	// +kubebuilder:default=true
	WaitForIt bool `json:"waitForIt"`
}
