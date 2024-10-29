package conditions

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type MockConditioner struct {
	conditions []metav1.Condition
}

func (m *MockConditioner) SetCondition(condition metav1.Condition) {
	m.conditions = append(m.conditions, condition)
}
