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

package e2e

import (
	"testing"

	api "github.com/nanosapp/vault-operator/pkg/apis/vault/v1alpha1"
	"github.com/nanosapp/vault-operator/test/e2e/e2eutil"
	"github.com/nanosapp/vault-operator/test/e2e/framework"
)

func TestUpgradeVault(t *testing.T) {
	f := framework.Global
	vaultCR := e2eutil.NewCluster("test-vault-", f.Namespace, 2)
	vaultCR.Spec.Version = "0.9.1-0"
	vaultCR, err := e2eutil.CreateCluster(t, f.VaultsCRClient, vaultCR)
	if err != nil {
		t.Fatalf("failed to create vault cluster: %v", err)
	}
	defer func(vaultCR *api.VaultService) {
		if err := e2eutil.DeleteCluster(t, f.VaultsCRClient, vaultCR); err != nil {
			t.Fatalf("failed to delete vault cluster: %v", err)
		}
	}(vaultCR)
	vaultCR, tlsConfig := e2eutil.WaitForCluster(t, f.KubeClient, f.VaultsCRClient, vaultCR)

	// Initialize vault via the 1st sealed node
	podName := vaultCR.Status.VaultStatus.Sealed[0]
	vClient := e2eutil.SetupVaultClient(t, f.KubeClient, f.Namespace, tlsConfig, podName)
	vaultCR, initResp := e2eutil.InitializeVault(t, f.VaultsCRClient, vaultCR, vClient)

	// Unseal the 1st vault node and wait for it to become active
	podName = vaultCR.Status.VaultStatus.Sealed[0]
	vClient = e2eutil.SetupVaultClient(t, f.KubeClient, f.Namespace, tlsConfig, podName)
	if err := e2eutil.UnsealVaultNode(initResp.Keys[0], vClient); err != nil {
		t.Fatalf("failed to unseal vault node(%v): %v", podName, err)
	}
	vaultCR, err = e2eutil.WaitActiveVaultsUp(t, f.VaultsCRClient, 6, vaultCR)
	if err != nil {
		t.Fatalf("failed to wait for any node to become active: %v", err)
	}

	// Unseal the 2nd vault node and wait for it to become standby
	podName = vaultCR.Status.VaultStatus.Sealed[0]
	vClient = e2eutil.SetupVaultClient(t, f.KubeClient, f.Namespace, tlsConfig, podName)
	if err = e2eutil.UnsealVaultNode(initResp.Keys[0], vClient); err != nil {
		t.Fatalf("failed to unseal vault node(%v): %v", podName, err)
	}
	vaultCR, err = e2eutil.WaitStandbyVaultsUp(t, f.VaultsCRClient, 1, 6, vaultCR)
	if err != nil {
		t.Fatalf("failed to wait for vault nodes to become standby: %v", err)
	}

	// Upgrade vault version
	newVersion := "0.9.1-1"
	vaultCR, err = e2eutil.UpdateVersion(t, f.VaultsCRClient, vaultCR, newVersion)
	if err != nil {
		t.Fatalf("failed to update vault version: %v", err)
	}

	// Check for 2 sealed nodes
	vaultCR, err = e2eutil.WaitSealedVaultsUp(t, f.VaultsCRClient, 2, 6, vaultCR)
	if err != nil {
		t.Fatalf("failed to wait for updated sealed vault nodes: %v", err)
	}

	podName = vaultCR.Status.VaultStatus.Sealed[0]
	vClient = e2eutil.SetupVaultClient(t, f.KubeClient, f.Namespace, tlsConfig, podName)
	if err = e2eutil.UnsealVaultNode(initResp.Keys[0], vClient); err != nil {
		t.Fatalf("failed to unseal vault node(%v): %v", podName, err)
	}
	podName = vaultCR.Status.VaultStatus.Sealed[1]
	vClient = e2eutil.SetupVaultClient(t, f.KubeClient, f.Namespace, tlsConfig, podName)
	if err = e2eutil.UnsealVaultNode(initResp.Keys[0], vClient); err != nil {
		t.Fatalf("failed to unseal vault node(%v): %v", podName, err)
	}

	upgradedNodes := vaultCR.Status.VaultStatus.Sealed

	// Check that the active node is one of the newly unsealed nodes
	vaultCR, err = e2eutil.WaitUntilActiveIsFrom(t, f.VaultsCRClient, 6, vaultCR, upgradedNodes...)
	if err != nil {
		t.Fatalf("failed to see the active node to be from the newly unsealed pods (%v): %v", upgradedNodes, err)
	}

	// Check that the standby node(s) are all from the newly unsealed nodes
	vaultCR, err = e2eutil.WaitUntilStandbyAreFrom(t, f.VaultsCRClient, 6, vaultCR, upgradedNodes...)
	if err != nil {
		t.Fatalf("failed to see all the standby nodes to be from the newly unsealed pods (%v): %v", upgradedNodes, err)
	}

	// Check that the available nodes are all from the newly unsealed nodes, i.e the old nodes are deleted
	vaultCR, err = e2eutil.WaitUntilAvailableAreFrom(t, f.VaultsCRClient, 6, vaultCR, upgradedNodes...)
	if err != nil {
		t.Fatalf("failed to see all available nodes to be from the newly unsealed pods (%v): %v", upgradedNodes, err)
	}

	// Check that 1 active and 1 standby are of the updated version
	err = e2eutil.CheckVersionReached(t, f.KubeClient, newVersion, 6, vaultCR, vaultCR.Status.VaultStatus.Active)
	if err != nil {
		t.Fatalf("failed to wait for active node to become updated: %v", err)
	}
	err = e2eutil.CheckVersionReached(t, f.KubeClient, newVersion, 6, vaultCR, vaultCR.Status.VaultStatus.Standby...)
	if err != nil {
		t.Fatalf("failed to wait for standby nodes to become updated: %v", err)
	}
}
