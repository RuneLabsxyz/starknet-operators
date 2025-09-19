package controller

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/runelabs-xyz/starknet-operators/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("StarknetRPC PodMonitor Controller", func() {
	Context("When reconciling a StarknetRPC resource with PodMonitor", func() {
		const (
			resourceName = "test-starknet-rpc-podmonitor"
			namespace    = "default"
			network      = "mainnet"
		)

		var (
			ctx         context.Context
			starknetRPC *v1alpha1.StarknetRPC
			pod         *corev1.Pod
		)

		BeforeEach(func() {
			ctx = context.Background()

			// Create a StarknetRPC resource
			starknetRPC = &v1alpha1.StarknetRPC{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "pathfinder.runelabs.xyz/v1alpha1",
					Kind:       "StarknetRPC",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: v1alpha1.StarknetRPCSpec{
					Network: network,
					RestoreArchive: v1alpha1.ArchiveSnapshot{
						Enable:   &[]bool{false}[0],
						FileName: "test-snapshot.tar",
						Checksum: "test-checksum",
						Storage: v1alpha1.StorageTemplate{
							Size: resource.MustParse("10Gi"),
						},
					},
					Storage: v1alpha1.StorageTemplate{
						Size: resource.MustParse("100Gi"),
					},
					Layer1RpcSecret: corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "l1-rpc-secret",
						},
						Key: "url",
					},
					PodMonitor: &v1alpha1.PodMonitor{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, starknetRPC)).Should(Succeed())

			// Create the Pod that the PodMonitor will target
			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-rpc", resourceName),
					Namespace: namespace,
					Labels: map[string]string{
						"rpc.runelabs.xyz/type": "starknet",
						"rpc.runelabs.xyz/name": resourceName,
						"runelabs.xyz/network":  network,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "rpc-pathfinder",
							Image: "eqlabs/pathfinder:v0.20.0",
							Ports: []corev1.ContainerPort{
								{
									Name:          "rpc",
									ContainerPort: 9545,
								},
								{
									Name:          "monitoring",
									ContainerPort: 9000,
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).Should(Succeed())
		})

		AfterEach(func() {
			// Clean up resources
			podMonitor := &monitoringv1.PodMonitor{}
			_ = k8sClient.Get(ctx, types.NamespacedName{
				Name:      fmt.Sprintf("%s-podmonitor", resourceName),
				Namespace: namespace,
			}, podMonitor)
			_ = k8sClient.Delete(ctx, podMonitor)

			_ = k8sClient.Delete(ctx, pod)
			_ = k8sClient.Delete(ctx, starknetRPC)
		})

		It("Should create a PodMonitor when enabled and Pod exists", func() {
			// Create reconciler
			reconciler := &StarknetRPCReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			// Reconcile PodMonitor
			result, err := reconciler.ReconcilePodMonitor(ctx, starknetRPC)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			// Verify PodMonitor was created
			podMonitor := &monitoringv1.PodMonitor{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      fmt.Sprintf("%s-podmonitor", resourceName),
				Namespace: namespace,
			}, podMonitor)
			Expect(err).NotTo(HaveOccurred())

			// Verify PodMonitor specifications
			Expect(podMonitor.Spec.PodMetricsEndpoints).To(HaveLen(1))
			Expect(*podMonitor.Spec.PodMetricsEndpoints[0].Port).To(Equal("monitoring"))
			Expect(podMonitor.Spec.PodMetricsEndpoints[0].Path).To(Equal("/metrics"))

			// Verify selector
			Expect(podMonitor.Spec.Selector.MatchLabels).To(HaveKeyWithValue("rpc.runelabs.xyz/name", resourceName))

			// Verify namespace selector
			Expect(podMonitor.Spec.NamespaceSelector.MatchNames).To(ContainElement(namespace))

			// Verify target labels
			Expect(podMonitor.Spec.PodTargetLabels).To(ContainElements(
				"runelabs.xyz/network",
				"rpc.runelabs.xyz/name",
			))

			// Verify labels
			Expect(podMonitor.Labels).To(HaveKeyWithValue("rpc.runelabs.xyz/type", "starknet"))
			Expect(podMonitor.Labels).To(HaveKeyWithValue("rpc.runelabs.xyz/name", resourceName))
			Expect(podMonitor.Labels).To(HaveKeyWithValue("runelabs.xyz/network", network))

			// Verify owner references
			Expect(podMonitor.OwnerReferences).To(HaveLen(1))
			Expect(podMonitor.OwnerReferences[0].Name).To(Equal(resourceName))
			Expect(podMonitor.OwnerReferences[0].Kind).To(Equal("StarknetRPC"))
		})

		It("Should not create a PodMonitor when enabled but Pod does not exist", func() {
			// Delete the pod first
			Expect(k8sClient.Delete(ctx, pod)).Should(Succeed())

			// Create reconciler
			reconciler := &StarknetRPCReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			// Reconcile PodMonitor
			result, err := reconciler.ReconcilePodMonitor(ctx, starknetRPC)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			// Verify PodMonitor was not created
			podMonitor := &monitoringv1.PodMonitor{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      fmt.Sprintf("%s-podmonitor", resourceName),
				Namespace: namespace,
			}, podMonitor)
			Expect(err).To(HaveOccurred())
			Expect(client.IgnoreNotFound(err)).To(BeNil())
		})

		It("Should update PodMonitor when specs change", func() {
			// Create reconciler
			reconciler := &StarknetRPCReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			// First reconciliation - create PodMonitor
			result, err := reconciler.ReconcilePodMonitor(ctx, starknetRPC)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			// Get the created PodMonitor
			podMonitor := &monitoringv1.PodMonitor{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      fmt.Sprintf("%s-podmonitor", resourceName),
				Namespace: namespace,
			}, podMonitor)
			Expect(err).NotTo(HaveOccurred())

			// Modify the PodMonitor to simulate drift
			podMonitor.Spec.PodMetricsEndpoints[0].Path = "/wrong-path"
			Expect(k8sClient.Update(ctx, podMonitor)).Should(Succeed())

			// Second reconciliation - should update PodMonitor
			result, err = reconciler.ReconcilePodMonitor(ctx, starknetRPC)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			// Verify PodMonitor was corrected
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      fmt.Sprintf("%s-podmonitor", resourceName),
				Namespace: namespace,
			}, podMonitor)
			Expect(err).NotTo(HaveOccurred())
			Expect(podMonitor.Spec.PodMetricsEndpoints[0].Path).To(Equal("/metrics"))
		})

		It("Should not create a PodMonitor when disabled", func() {
			// Update StarknetRPC to disable PodMonitor
			starknetRPC.Spec.PodMonitor = &v1alpha1.PodMonitor{
				Enabled: false,
			}

			// Create reconciler
			reconciler := &StarknetRPCReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			// Reconcile PodMonitor
			result, err := reconciler.ReconcilePodMonitor(ctx, starknetRPC)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			// Verify PodMonitor was not created
			podMonitor := &monitoringv1.PodMonitor{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      fmt.Sprintf("%s-podmonitor", resourceName),
				Namespace: namespace,
			}, podMonitor)
			Expect(err).To(HaveOccurred())
			Expect(client.IgnoreNotFound(err)).To(BeNil())
		})

		It("Should not create a PodMonitor when PodMonitor field is nil", func() {
			// Update StarknetRPC to have nil PodMonitor (default)
			starknetRPC.Spec.PodMonitor = nil

			// Create reconciler
			reconciler := &StarknetRPCReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			// Reconcile PodMonitor
			result, err := reconciler.ReconcilePodMonitor(ctx, starknetRPC)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			// Verify PodMonitor was not created
			podMonitor := &monitoringv1.PodMonitor{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      fmt.Sprintf("%s-podmonitor", resourceName),
				Namespace: namespace,
			}, podMonitor)
			Expect(err).To(HaveOccurred())
			Expect(client.IgnoreNotFound(err)).To(BeNil())
		})

		It("Should delete existing PodMonitor when monitoring is disabled", func() {
			// Create reconciler
			reconciler := &StarknetRPCReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			// First reconciliation - create PodMonitor (enabled)
			result, err := reconciler.ReconcilePodMonitor(ctx, starknetRPC)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			// Verify PodMonitor was created
			podMonitor := &monitoringv1.PodMonitor{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      fmt.Sprintf("%s-podmonitor", resourceName),
				Namespace: namespace,
			}, podMonitor)
			Expect(err).NotTo(HaveOccurred())

			// Now disable monitoring
			starknetRPC.Spec.PodMonitor.Enabled = false

			// Second reconciliation - should delete PodMonitor
			result, err = reconciler.ReconcilePodMonitor(ctx, starknetRPC)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			// Verify PodMonitor was deleted
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      fmt.Sprintf("%s-podmonitor", resourceName),
				Namespace: namespace,
			}, podMonitor)
			Expect(err).To(HaveOccurred())
			Expect(client.IgnoreNotFound(err)).To(BeNil())
		})

		It("Should add custom labels to PodMonitor when provided", func() {
			// Add custom labels to the PodMonitor config
			starknetRPC.Spec.PodMonitor.Labels = map[string]string{
				"custom-label-1": "value-1",
				"custom-label-2": "value-2",
				"prometheus":     "enabled",
			}

			// Create reconciler
			reconciler := &StarknetRPCReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: record.NewFakeRecorder(10),
			}

			// Reconcile PodMonitor
			result, err := reconciler.ReconcilePodMonitor(ctx, starknetRPC)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			// Verify PodMonitor was created with custom labels
			podMonitor := &monitoringv1.PodMonitor{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      fmt.Sprintf("%s-podmonitor", resourceName),
				Namespace: namespace,
			}, podMonitor)
			Expect(err).NotTo(HaveOccurred())

			// Verify custom labels are present
			Expect(podMonitor.Labels).To(HaveKeyWithValue("custom-label-1", "value-1"))
			Expect(podMonitor.Labels).To(HaveKeyWithValue("custom-label-2", "value-2"))
			Expect(podMonitor.Labels).To(HaveKeyWithValue("prometheus", "enabled"))

			// Also verify default labels are still there
			Expect(podMonitor.Labels).To(HaveKeyWithValue("rpc.runelabs.xyz/type", "starknet"))
			Expect(podMonitor.Labels).To(HaveKeyWithValue("rpc.runelabs.xyz/name", resourceName))
		})
	})

	Context("PodMonitor reconciler functions", func() {
		const (
			resourceName = "test-functions"
			namespace    = "default"
			network      = "testnet"
		)

		var starknetRPC *v1alpha1.StarknetRPC

		BeforeEach(func() {
			starknetRPC = &v1alpha1.StarknetRPC{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
					UID:       "test-uid",
				},
				Spec: v1alpha1.StarknetRPCSpec{
					Network: network,
					PodMonitor: &v1alpha1.PodMonitor{
						Enabled: true,
					},
				},
			}
		})

		Describe("GetPodMonitorName", func() {
			It("Should return correct name and namespace", func() {
				reconciler := &StarknetRPCReconciler{}
				namespacedName := reconciler.GetPodMonitorName(starknetRPC)

				Expect(namespacedName.Name).To(Equal(fmt.Sprintf("%s-podmonitor", resourceName)))
				Expect(namespacedName.Namespace).To(Equal(namespace))
			})
		})

		Describe("GetWantedPodMonitor", func() {
			It("Should create PodMonitor with correct specifications", func() {
				reconciler := &StarknetRPCReconciler{}
				podMonitor := reconciler.GetWantedPodMonitor(starknetRPC)

				// Check metadata
				Expect(podMonitor.Name).To(Equal(fmt.Sprintf("%s-podmonitor", resourceName)))
				Expect(podMonitor.Namespace).To(Equal(namespace))

				// Check labels
				Expect(podMonitor.Labels).To(HaveKeyWithValue("rpc.runelabs.xyz/type", "starknet"))
				Expect(podMonitor.Labels).To(HaveKeyWithValue("rpc.runelabs.xyz/name", resourceName))
				Expect(podMonitor.Labels).To(HaveKeyWithValue("runelabs.xyz/network", network))
				Expect(podMonitor.Labels).To(HaveKeyWithValue("app.kubernetes.io/name", "starknet-rpc"))
				Expect(podMonitor.Labels).To(HaveKeyWithValue("app.kubernetes.io/instance", resourceName))
				Expect(podMonitor.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "starknet-operator"))

				// Check owner references
				Expect(podMonitor.OwnerReferences).To(HaveLen(1))
				Expect(podMonitor.OwnerReferences[0].Name).To(Equal(resourceName))
				Expect(podMonitor.OwnerReferences[0].UID).To(Equal(starknetRPC.UID))
				Expect(*podMonitor.OwnerReferences[0].Controller).To(BeTrue())
				Expect(*podMonitor.OwnerReferences[0].BlockOwnerDeletion).To(BeTrue())

				// Check spec
				Expect(podMonitor.Spec.PodMetricsEndpoints).To(HaveLen(1))
				Expect(*podMonitor.Spec.PodMetricsEndpoints[0].Port).To(Equal("monitoring"))
				Expect(podMonitor.Spec.PodMetricsEndpoints[0].Path).To(Equal("/metrics"))

				// Check selector
				Expect(podMonitor.Spec.Selector.MatchLabels).To(HaveKeyWithValue("rpc.runelabs.xyz/name", resourceName))

				// Check namespace selector
				Expect(podMonitor.Spec.NamespaceSelector.MatchNames).To(ContainElement(namespace))

				// Check target labels
				Expect(podMonitor.Spec.PodTargetLabels).To(ContainElements(
					"runelabs.xyz/network",
					"rpc.runelabs.xyz/name",
				))
			})

			It("Should include custom labels when provided", func() {
				starknetRPC.Spec.PodMonitor.Labels = map[string]string{
					"team":        "platform",
					"environment": "production",
				}

				reconciler := &StarknetRPCReconciler{}
				podMonitor := reconciler.GetWantedPodMonitor(starknetRPC)

				// Check custom labels are included
				Expect(podMonitor.Labels).To(HaveKeyWithValue("team", "platform"))
				Expect(podMonitor.Labels).To(HaveKeyWithValue("environment", "production"))

				// Check default labels are still present
				Expect(podMonitor.Labels).To(HaveKeyWithValue("rpc.runelabs.xyz/type", "starknet"))
				Expect(podMonitor.Labels).To(HaveKeyWithValue("rpc.runelabs.xyz/name", resourceName))
			})
		})

		Describe("PodMonitorSpecReconciler", func() {
			It("Should detect when PodMonitor is up to date", func() {
				reconciler := PodMonitorSpecReconciler(starknetRPC)
				portName := "monitoring"

				podMonitor := &monitoringv1.PodMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"rpc.runelabs.xyz/type":        "starknet",
							"rpc.runelabs.xyz/name":        resourceName,
							"runelabs.xyz/network":         network,
							"app.kubernetes.io/name":       "starknet-rpc",
							"app.kubernetes.io/instance":   resourceName,
							"app.kubernetes.io/managed-by": "starknet-operator",
						},
					},
					Spec: monitoringv1.PodMonitorSpec{
						PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
							{
								Port: &portName,
								Path: "/metrics",
							},
						},
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"rpc.runelabs.xyz/name": resourceName,
							},
						},
						NamespaceSelector: monitoringv1.NamespaceSelector{
							MatchNames: []string{namespace},
						},
						PodTargetLabels: []string{
							"runelabs.xyz/network",
							"rpc.runelabs.xyz/name",
						},
					},
				}

				Expect(reconciler.IsUpToDate(podMonitor)).To(BeTrue())
			})

			It("Should detect when PodMonitor needs update", func() {
				reconciler := PodMonitorSpecReconciler(starknetRPC)
				portName := "wrong-port"

				podMonitor := &monitoringv1.PodMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"rpc.runelabs.xyz/type":        "starknet",
							"rpc.runelabs.xyz/name":        resourceName,
							"runelabs.xyz/network":         network,
							"app.kubernetes.io/name":       "starknet-rpc",
							"app.kubernetes.io/instance":   resourceName,
							"app.kubernetes.io/managed-by": "starknet-operator",
						},
					},
					Spec: monitoringv1.PodMonitorSpec{
						PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
							{
								Port: &portName,
								Path: "/metrics",
							},
						},
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"rpc.runelabs.xyz/name": resourceName,
							},
						},
					},
				}

				Expect(reconciler.IsUpToDate(podMonitor)).To(BeFalse())
			})

			It("Should update PodMonitor spec correctly", func() {
				reconciler := PodMonitorSpecReconciler(starknetRPC)

				podMonitor := &monitoringv1.PodMonitor{
					Spec: monitoringv1.PodMonitorSpec{},
				}

				err := reconciler.Update(podMonitor)
				Expect(err).NotTo(HaveOccurred())

				Expect(podMonitor.Spec.PodMetricsEndpoints).To(HaveLen(1))
				Expect(*podMonitor.Spec.PodMetricsEndpoints[0].Port).To(Equal("monitoring"))
				Expect(podMonitor.Spec.PodMetricsEndpoints[0].Path).To(Equal("/metrics"))
				Expect(podMonitor.Spec.Selector.MatchLabels).To(HaveKeyWithValue("rpc.runelabs.xyz/name", resourceName))
			})
		})
	})
})
