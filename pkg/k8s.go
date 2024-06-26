// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package pkg

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	ipamv1alpha1 "github.com/ironcore-dev/ipam/api/ipam/v1alpha1"
	inventoryv1alpha4 "github.com/ironcore-dev/metal/apis/metal/v1alpha4"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type K8sClient struct {
	Client        client.Client
	EventRecorder record.EventRecorder
}

func NewK8sClient(cfg *rest.Config, options client.Options) K8sClient {
	if err := inventoryv1alpha4.AddToScheme(scheme.Scheme); err != nil {
		log.Fatal("Unable to add registered types inventory to client scheme: ", err)
	}
	if err := ipamv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		log.Fatal("Unable to add registered types ipam to client scheme: ", err)
	}

	if cfg == nil {
		cfg = config.GetConfigOrDie()
	}

	cl, err := client.New(cfg, options)
	if err != nil {
		log.Fatal("Failed to create a controller runtime client: ", err)
	}

	corev1Client, err := corev1client.NewForConfig(cfg)
	if err != nil {
		log.Fatal("Failed to create a core client: ", err)
	}

	broadcaster := record.NewBroadcaster()

	// Leader id, needs to be unique
	id, err := os.Hostname()
	if err != nil {
		log.Fatal("Failed to get hostname: ", err)
	}
	recorder := broadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: id})
	broadcaster.StartRecordingToSink(&corev1client.EventSinkImpl{Interface: corev1Client.Events("")})

	return K8sClient{
		Client:        cl,
		EventRecorder: recorder,
	}
}

func (k K8sClient) getSecret(name, namespace string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	err := k.Client.Get(context.Background(), client.ObjectKeyFromObject(secret), secret)
	if err != nil {
		log.Printf("Failed to get Secret %s in Namespace %s: %s", name, namespace, err)
		return nil, err
	}

	return secret, nil
}

func (k K8sClient) getConfigMag(name, namespace string) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	err := k.Client.Get(context.Background(), client.ObjectKeyFromObject(configMap), configMap)
	if err != nil {
		log.Printf("Failed to get ConfigMap %s in Namespace %s: %s", name, namespace, err)
		return nil, err
	}

	return configMap, nil
}

func (k K8sClient) getMacFromIP(clientIP, namespace string) (string, error) {
	if getIPVersion(clientIP) == "ipv6" {
		ip := net.ParseIP(clientIP)
		clientIP = getLongIPv6(ip)
	}

	var ips ipamv1alpha1.IPList
	err := k.Client.List(context.Background(),
		&ips,
		client.InNamespace(namespace),
		client.MatchingLabels{"ip": strings.ReplaceAll(clientIP, ":", "-")})
	if err != nil {
		err = errors.Wrapf(err, "Failed to list IPAM IPs in namespace %s", namespace)
		return "", err
	}

	var mac string
	if len(ips.Items) == 0 {
		return "", errors.New(fmt.Sprintf("IP %s is unknown", clientIP))
	} else if len(ips.Items) > 1 {
		return "", errors.New(fmt.Sprintf("More than one IP %s found", clientIP))
	} else if len(ips.Items) == 1 {
		macLabel, exists := ips.Items[0].Labels["mac"]
		if !exists {
			return "", errors.New(fmt.Sprintf("No Mac was found for IP %s", clientIP))
		}
		mac = macLabel
	}

	log.Printf("Mac %s for IPAM IP %s found", mac, clientIP)
	return mac, nil
}

func (k K8sClient) getInventory(uuid, namespace string) (*inventoryv1alpha4.Inventory, error) {

	inventory := &inventoryv1alpha4.Inventory{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uuid,
			Namespace: namespace,
		},
	}
	err := k.Client.Get(context.Background(), client.ObjectKeyFromObject(inventory), inventory)
	if err != nil {
		err = errors.Wrapf(err, "Failed to get inventory in namespace %s", namespace)
		return nil, err
	}

	log.Printf("Found Inventory for UUID %s", uuid)
	return inventory, nil
}
