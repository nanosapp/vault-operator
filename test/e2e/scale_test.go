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

func TestScaleUp(t *testing.T) {
	f := framework.Global
	vaultCR, err := e2eutil.CreateCluster(t, f.VaultsCRClient, e2eutil.NewCluster("test-vault-", f.Namespace, 1))
	if err != nil {
		t.Fatalf("failed to create vault cluster: %v", err)
	}
	defer func(vaultCR *api.VaultService) {
		if err := e2eutil.DeleteCluster(t, f.VaultsCRClient, vaultCR); err != nil {
			t.Fatalf("failed to delete vault cluster: %v", err)
		}
	}(vaultCR)

	vaultCR, tlsConfig := e2eutil.WaitForCluster(t, f.KubeClient, f.VaultsCRClient, vaultCR)

	// Init vault via the first sealed node
	podName := vaultCR.Status.VaultStatus.Sealed[0]
	vClient := e2eutil.SetupVaultClient(t, f.KubeClient, f.Namespace, tlsConfig, podName)
	vaultCR, initResp := e2eutil.InitializeVault(t, f.VaultsCRClient, vaultCR, vClient)

	// Unseal the vault node and wait for it to become active
	podName = vaultCR.Status.VaultStatus.Sealed[0]
	vClient = e2eutil.SetupVaultClient(t, f.KubeClient, f.Namespace, tlsConfig, podName)
	if err := e2eutil.UnsealVaultNode(initResp.Keys[0], vClient); err != nil {
		t.Fatalf("failed to unseal vault node(%v): %v", podName, err)
	}
	vaultCR, err = e2eutil.WaitActiveVaultsUp(t, f.VaultsCRClient, 6, vaultCR)
	if err != nil {
		t.Fatalf("failed to wait for any node to become active: %v", err)
	}

	// TODO: Write secret to active node, read secret from new node later

	// Resize cluster to 2 nodes
	vaultCR, err = e2eutil.ResizeCluster(t, f.VaultsCRClient, vaultCR, 2)
	if err != nil {
		t.Fatalf("failed to resize vault cluster: %v", err)
	}

	// Wait for 1 unsealed node and create a vault client for it
	vaultCR, err = e2eutil.WaitSealedVaultsUp(t, f.VaultsCRClient, 1, 6, vaultCR)
	if err != nil {
		t.Fatalf("failed to wait for vault nodes to become sealed: %v", err)
	}

	podName = vaultCR.Status.VaultStatus.Sealed[0]
	// Unseal the new node and wait for it to become standby
	vClient = e2eutil.SetupVaultClient(t, f.KubeClient, f.Namespace, tlsConfig, podName)
	if err := e2eutil.UnsealVaultNode(initResp.Keys[0], vClient); err != nil {
		t.Fatalf("failed to unseal vault node(%v): %v", podName, err)
	}
	vaultCR, err = e2eutil.WaitStandbyVaultsUp(t, f.VaultsCRClient, 1, 6, vaultCR)
	if err != nil {
		t.Fatalf("failed to wait for vault nodes to become standby: %v", err)
	}

}
