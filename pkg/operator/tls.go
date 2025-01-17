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

package operator

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"

	api "github.com/nanosapp/vault-operator/pkg/apis/vault/v1alpha1"
	"github.com/nanosapp/vault-operator/pkg/util/k8sutil"
	"github.com/nanosapp/vault-operator/pkg/util/tlsutil"
	"github.com/nanosapp/vault-operator/pkg/util/vaultutil"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	orgForTLSCert        = []string{"coreos.com"}
	defaultClusterDomain = "cluster.local"
)

// prepareDefaultVaultTLSSecrets creates the default secrets for the vault server's TLS assets.
// Currently we self-generate the CA, and use the self generated CA to sign all the TLS certs.
func (v *Vaults) prepareDefaultVaultTLSSecrets(vr *api.VaultService) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("prepare default vault TLS secrets failed: %v", err)
		}
	}()

	// if TLS spec doesn't exist or secrets doesn't exist, then we can go create secrets.
	// TODO: we won't need IsTLSConfigured() check once we have initializers.
	if api.IsTLSConfigured(vr.Spec.TLS) {
		// TODO: use secrets informer
		_, err = v.kubecli.CoreV1().Secrets(vr.Namespace).Get(vr.Spec.TLS.Static.ServerSecret, metav1.GetOptions{})
		if err == nil {
			return nil
		}
		if !apierrors.IsNotFound(err) {
			return err
		}
	}

	// TODO: optional user pass-in CA.
	caKey, caCrt, err := newCACert()
	if err != nil {
		return err
	}

	se, err := newVaultServerTLSSecret(vr, caKey, caCrt)
	if err != nil {
		return err
	}
	k8sutil.AddOwnerRefToObject(se, k8sutil.AsOwner(vr))
	_, err = v.kubecli.CoreV1().Secrets(vr.Namespace).Create(se)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	se = newVaultClientTLSSecret(vr, caCrt)
	k8sutil.AddOwnerRefToObject(se, k8sutil.AsOwner(vr))
	_, err = v.kubecli.CoreV1().Secrets(vr.Namespace).Create(se)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// prepareEtcdTLSSecrets creates three etcd TLS secrets (client, server, peer) containing TLS assets.
// Currently we self-generate the CA, and use the self generated CA to sign all the TLS certs.
func (v *Vaults) prepareEtcdTLSSecrets(vr *api.VaultService) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("prepare TLS secrets failed: %v", err)
		}
	}()

	// TODO: use secrets informer
	_, err = v.kubecli.CoreV1().Secrets(vr.Namespace).Get(k8sutil.EtcdClientTLSSecretName(vr.Name), metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	// TODO: optional user pass-in CA.
	caKey, caCrt, err := newCACert()
	if err != nil {
		return err
	}

	se, err := newEtcdClientTLSSecret(vr, caKey, caCrt)
	if err != nil {
		return err
	}
	k8sutil.AddOwnerRefToObject(se, k8sutil.AsOwner(vr))
	_, err = v.kubecli.CoreV1().Secrets(vr.Namespace).Create(se)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	se, err = newEtcdServerTLSSecret(vr, caKey, caCrt)
	if err != nil {
		return err
	}
	k8sutil.AddOwnerRefToObject(se, k8sutil.AsOwner(vr))
	_, err = v.kubecli.CoreV1().Secrets(vr.Namespace).Create(se)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	se, err = newEtcdPeerTLSSecret(vr, caKey, caCrt)
	if err != nil {
		return err
	}
	k8sutil.AddOwnerRefToObject(se, k8sutil.AsOwner(vr))
	_, err = v.kubecli.CoreV1().Secrets(vr.Namespace).Create(se)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// cleanupTLSSecrets cleans up etcd TLS secrets generated by operator for the given vault.
func (v *Vaults) cleanupEtcdTLSSecrets(vr *api.VaultService) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cleanup TLS secrets failed: %v", err)
		}
	}()
	name := k8sutil.EtcdClientTLSSecretName(vr.Name)
	err = v.kubecli.CoreV1().Secrets(vr.Namespace).Delete(name, nil)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete secret (%s) failed: %v", name, err)
	}

	name = k8sutil.EtcdServerTLSSecretName(vr.Name)
	err = v.kubecli.CoreV1().Secrets(vr.Namespace).Delete(name, nil)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete secret (%s) failed: %v", name, err)
	}

	name = k8sutil.EtcdPeerTLSSecretName(vr.Name)
	err = v.kubecli.CoreV1().Secrets(vr.Namespace).Delete(name, nil)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete secret (%s) failed: %v", name, err)
	}
	return nil
}

// newEtcdClientTLSSecret returns a secret containing etcd client TLS assets
func newEtcdClientTLSSecret(vr *api.VaultService, caKey *rsa.PrivateKey, caCrt *x509.Certificate) (*v1.Secret, error) {
	return newTLSSecret(vr, caKey, caCrt, "etcd client", k8sutil.EtcdClientTLSSecretName(vr.Name), nil,
		map[string]string{
			"key":  "etcd-client.key",
			"cert": "etcd-client.crt",
			"ca":   "etcd-client-ca.crt",
		})
}

// newEtcdServerTLSSecret returns a secret containing etcd server TLS assets
func newEtcdServerTLSSecret(vr *api.VaultService, caKey *rsa.PrivateKey, caCrt *x509.Certificate) (*v1.Secret, error) {
	return newTLSSecret(vr, caKey, caCrt, "etcd server", k8sutil.EtcdServerTLSSecretName(vr.Name),
		[]string{
			"localhost",
			fmt.Sprintf("*.%s.%s.svc", k8sutil.EtcdNameForVault(vr.Name), vr.Namespace),
			fmt.Sprintf("%s-client", k8sutil.EtcdNameForVault(vr.Name)),
			fmt.Sprintf("%s-client.%s", k8sutil.EtcdNameForVault(vr.Name), vr.Namespace),
			fmt.Sprintf("%s-client.%s.svc", k8sutil.EtcdNameForVault(vr.Name), vr.Namespace),
			// TODO: get rid of cluster domain
			fmt.Sprintf("*.%s.%s.svc.%s", k8sutil.EtcdNameForVault(vr.Name), vr.Namespace, defaultClusterDomain),
			fmt.Sprintf("%s-client.%s.svc.%s", k8sutil.EtcdNameForVault(vr.Name), vr.Namespace, defaultClusterDomain),
		},
		map[string]string{
			"key":  "server.key",
			"cert": "server.crt",
			"ca":   "server-ca.crt",
		})
}

// newEtcdPeerTLSSecret returns a secret containing etcd peer TLS assets
func newEtcdPeerTLSSecret(vr *api.VaultService, caKey *rsa.PrivateKey, caCrt *x509.Certificate) (*v1.Secret, error) {
	return newTLSSecret(vr, caKey, caCrt, "etcd peer", k8sutil.EtcdPeerTLSSecretName(vr.Name),
		[]string{
			fmt.Sprintf("*.%s.%s.svc", k8sutil.EtcdNameForVault(vr.Name), vr.Namespace),
			// TODO: get rid of cluster domain
			fmt.Sprintf("*.%s.%s.svc.%s", k8sutil.EtcdNameForVault(vr.Name), vr.Namespace, defaultClusterDomain),
		},
		map[string]string{
			"key":  "peer.key",
			"cert": "peer.crt",
			"ca":   "peer-ca.crt",
		})
}

// cleanupDefaultVaultTLSSecrets cleans up any auto generated vault TLS secrets for the given vault cluster
func (v *Vaults) cleanupDefaultVaultTLSSecrets(vr *api.VaultService) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("cleanup vault TLS secrets failed: %v", err)
		}
	}()
	name := api.DefaultVaultServerTLSSecretName(vr.Name)
	err = v.kubecli.CoreV1().Secrets(vr.Namespace).Delete(name, nil)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete secret (%s) failed: %v", name, err)
	}

	name = api.DefaultVaultClientTLSSecretName(vr.Name)
	err = v.kubecli.CoreV1().Secrets(vr.Namespace).Delete(name, nil)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete secret (%s) failed: %v", name, err)
	}
	return nil
}

// newVaultServerTLSSecret returns a secret containing vault server TLS assets
func newVaultServerTLSSecret(vr *api.VaultService, caKey *rsa.PrivateKey, caCrt *x509.Certificate) (*v1.Secret, error) {
	return newTLSSecret(vr, caKey, caCrt, "vault server", api.DefaultVaultServerTLSSecretName(vr.Name),
		[]string{
			"localhost",
			fmt.Sprintf("*.%s.pod", vr.Namespace),
			fmt.Sprintf("%s.%s.svc", vr.Name, vr.Namespace),
		},
		map[string]string{
			"key":  vaultutil.ServerTLSKeyName,
			"cert": vaultutil.ServerTLSCertName,
			// The CA is not used by the server
			"ca": "server-ca.crt",
		})
}

// newVaultClientTLSSecret returns a secret containing vault client TLS assets.
// The client key and certificate are not generated since clients are not authenticated at the server
func newVaultClientTLSSecret(vr *api.VaultService, caCrt *x509.Certificate) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   api.DefaultVaultClientTLSSecretName(vr.Name),
			Labels: k8sutil.LabelsForVault(vr.Name),
		},
		Data: map[string][]byte{
			api.CATLSCertName: tlsutil.EncodeCertificatePEM(caCrt),
		},
	}
}

// newTLSSecret is a common utility for creating a secret containing TLS assets.
func newTLSSecret(vr *api.VaultService, caKey *rsa.PrivateKey, caCrt *x509.Certificate, commonName, secretName string,
	addrs []string, fieldMap map[string]string) (*v1.Secret, error) {
	tc := tlsutil.CertConfig{
		CommonName:   commonName,
		Organization: orgForTLSCert,
		AltNames:     tlsutil.NewAltNames(addrs),
	}
	key, crt, err := newKeyAndCert(caCrt, caKey, tc)
	if err != nil {
		return nil, fmt.Errorf("new TLS secret failed: %v", err)
	}
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   secretName,
			Labels: k8sutil.LabelsForVault(vr.Name),
		},
		Data: map[string][]byte{
			fieldMap["key"]:  tlsutil.EncodePrivateKeyPEM(key),
			fieldMap["cert"]: tlsutil.EncodeCertificatePEM(crt),
			fieldMap["ca"]:   tlsutil.EncodeCertificatePEM(caCrt),
		},
	}
	return secret, nil
}

func newCACert() (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	config := tlsutil.CertConfig{
		CommonName:   "vault operator CA",
		Organization: orgForTLSCert,
	}

	cert, err := tlsutil.NewSelfSignedCACertificate(config, key)
	if err != nil {
		return nil, nil, err
	}

	return key, cert, err
}

func newKeyAndCert(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, config tlsutil.CertConfig) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}
	// TODO: tlsutil.NewSignedCertificate()create certs for both client and server auth. We can limit it stricter.
	cert, err := tlsutil.NewSignedCertificate(config, key, caCert, caPrivKey)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, nil
}
