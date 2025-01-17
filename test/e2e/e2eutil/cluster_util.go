// Copyright 2018 The vault-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2eutil

import (
	"fmt"
	"testing"

	api "github.com/nanosapp/vault-operator/pkg/apis/vault/v1alpha1"
	"github.com/nanosapp/vault-operator/pkg/generated/clientset/versioned"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateCluster creates a vault CR with the desired spec
func CreateCluster(t *testing.T, crClient versioned.Interface, vs *api.VaultService) (*api.VaultService, error) {
	vault, err := crClient.VaultV1alpha1().VaultServices(vs.Namespace).Create(vs)
	if err != nil {
		return nil, fmt.Errorf("failed to create CR: %v", err)
	}
	LogfWithTimestamp(t, "created vault cluster: %s", vault.Name)
	return vault, nil
}

// ResizeCluster updates the Nodes field of the vault CR
func ResizeCluster(t *testing.T, crClient versioned.Interface, vs *api.VaultService, size int) (*api.VaultService, error) {
	vault, err := crClient.VaultV1alpha1().VaultServices(vs.Namespace).Get(vs.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get CR: %v", err)
	}
	vault.Spec.Nodes = int32(size)
	vault, err = crClient.VaultV1alpha1().VaultServices(vs.Namespace).Update(vault)
	if err != nil {
		return nil, fmt.Errorf("failed to update CR: %v", err)
	}
	LogfWithTimestamp(t, "updated vault cluster(%v) to size(%v)", vault.Name, size)
	return vault, nil
}

// UpdateVersion updates the Version field of the vault CR
func UpdateVersion(t *testing.T, crClient versioned.Interface, vs *api.VaultService, version string) (*api.VaultService, error) {
	vault, err := crClient.VaultV1alpha1().VaultServices(vs.Namespace).Get(vs.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get CR: %v", err)
	}
	vault.Spec.Version = version
	vault, err = crClient.VaultV1alpha1().VaultServices(vs.Namespace).Update(vault)
	if err != nil {
		return nil, fmt.Errorf("failed to update CR: %v", err)
	}
	LogfWithTimestamp(t, "updated vault cluster(%v) to version(%v)", vault.Name, version)
	return vault, nil
}

// DeleteCluster deletes the vault CR specified by cluster spec
func DeleteCluster(t *testing.T, crClient versioned.Interface, vs *api.VaultService) error {
	t.Logf("deleting vault cluster: %v", vs.Name)
	err := crClient.VaultV1alpha1().VaultServices(vs.Namespace).Delete(vs.Name, nil)
	if err != nil {
		return fmt.Errorf("failed to delete CR: %v", err)
	}
	// TODO: Wait for cluster resources to be deleted
	return nil
}
