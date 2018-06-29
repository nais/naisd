package api

import (
	"github.com/nais/naisd/api/app"
	"k8s.io/api/rbac/v1"
	k8smeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c clientHolder) createOrUpdateRoleBinding(subject app.Spec, roleRef v1.RoleRef) (*v1.RoleBinding, error) {
	roleBindingInterface := c.client.RbacV1().RoleBindings(subject.Namespace())
	def := createRoleBindingDef(subject, roleRef)

	if _, err := roleBindingInterface.Get(subject.ResourceName(), k8smeta.GetOptions{}); err == nil {
		return roleBindingInterface.Update(def)
	} else {
		return roleBindingInterface.Create(def)
	}
}

func createRoleRef(kind, name string) v1.RoleRef {
	return v1.RoleRef{
		Kind:     kind,
		Name:     name,
		APIGroup: "rbac.authorization.k8s.io",
	}
}

func createRoleBindingDef(subject app.Spec, roleRef v1.RoleRef) *v1.RoleBinding {
	return &v1.RoleBinding{
		ObjectMeta: generateObjectMeta(subject),
		Subjects: []v1.Subject{{
			Kind:      "ServiceAccount",
			Name:      subject.ResourceName(),
			Namespace: subject.Namespace(),
			APIGroup:  "rbac.authorization.k8s.io",
		}},
		RoleRef: roleRef,
	}
}
