/*
Copyright © contributors to CloudNativePG, established as
CloudNativePG a Series of LF Projects, LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

SPDX-License-Identifier: Apache-2.0
*/

// Package proxy provides functions to use the proxy subresource to call a pod
package proxy

import (
	"context"
	"strconv"

	"github.com/runelabs-xyz/starknet-operators/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// runProxyRequest makes a GET call on the pod interface proxy, and returns the raw response
func runProxyRequest(
	ctx context.Context,
	kubeInterface kubernetes.Interface,
	pod *corev1.Pod,
	tlsEnabled bool,
	path string,
	port int,
) ([]byte, error) {
	portString := strconv.Itoa(port)

	schema := "http"
	if tlsEnabled {
		schema = "https"
	}

	req := kubeInterface.CoreV1().Pods(pod.Namespace).ProxyGet(
		schema, pod.Name, portString, path, map[string]string{})

	return req.DoRaw(ctx)
}

func getNamedPort(pod *corev1.Pod, name string) int {
	for _, port := range pod.Spec.Containers[0].Ports {
		if port.Name == name {
			return int(port.ContainerPort)
		}
	}
	return 0
}

func IsReady(ctx context.Context, kubeInterface kubernetes.Interface, rpc *v1alpha1.StarknetRPC, pod *corev1.Pod) (bool, error) {
	// Find named port "management"
	port := strconv.Itoa(getNamedPort(pod, "management"))
	schema := "http"
	if pod.Spec.Containers[0].Ports[0].Protocol == corev1.ProtocolTCP {
		schema = "https"
	}

	req := kubeInterface.CoreV1().Pods(pod.Namespace).ProxyGet(
		schema, pod.Name, port, "/ready", map[string]string{})

	_, err := req.DoRaw(ctx)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get ready status")
		return false, err
	}

	return true, nil
}

func IsSynced(ctx context.Context, kubeInterface kubernetes.Interface, rpc *v1alpha1.StarknetRPC, pod *corev1.Pod) (bool, error) {
	// Find named port "management"
	port := strconv.Itoa(getNamedPort(pod, "management"))
	schema := "http"
	if pod.Spec.Containers[0].Ports[0].Protocol == corev1.ProtocolTCP {
		schema = "https"
	}

	req := kubeInterface.CoreV1().Pods(pod.Namespace).ProxyGet(
		schema, pod.Name, port, "/ready/synced", map[string]string{})

	_, err := req.DoRaw(ctx)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to get ready status")
		return false, err
	}

	return true, nil
}
