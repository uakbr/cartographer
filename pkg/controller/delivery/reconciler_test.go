// Copyright 2021 VMware
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

package delivery_test

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/vmware-tanzu/cartographer/pkg/apis/v1alpha1"
	"github.com/vmware-tanzu/cartographer/pkg/controller/delivery"
	"github.com/vmware-tanzu/cartographer/pkg/repository/repositoryfakes"
)

var _ = Describe("delivery reconciler", func() {
	var (
		repo       *repositoryfakes.FakeRepository
		reconciler reconcile.Reconciler
		ctx        context.Context
		req        reconcile.Request
		out        *Buffer
	)

	BeforeEach(func() {
		repo = &repositoryfakes.FakeRepository{}
		reconciler = delivery.NewReconciler(repo)

		out = NewBuffer()
		logger := zap.New(zap.WriteTo(out))
		ctx = logr.NewContext(context.Background(), logger)

		req = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: "my-new-delivery",
			},
		}
	})

	Context("with a well formed delivery", func() {
		var (
			apiDelivery *v1alpha1.ClusterDelivery
		)
		BeforeEach(func() {
			apiDelivery = &v1alpha1.ClusterDelivery{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "my-new-delivery",
					Generation: 99,
				},
				Spec: v1alpha1.ClusterDeliverySpec{
					Resources: []v1alpha1.ClusterDeliveryResource{
						{
							Name: "first-resource",
							TemplateRef: v1alpha1.DeliveryClusterTemplateReference{
								Kind: "ClusterSourceTemplate",
								Name: "my-source-template",
							},
						},
						{
							Name: "second-resource",
							TemplateRef: v1alpha1.DeliveryClusterTemplateReference{
								Kind: "ClusterTemplate",
								Name: "my-final-template",
							},
						},
					},
				},
			}
			repo.GetDeliveryReturns(apiDelivery, nil)
		})

		Context("all referenced templates exist", func() {
			BeforeEach(func() {
				// Returns no error, so template is `found`
				repo.GetDeliveryClusterTemplateReturns(nil, nil)
			})

			It("Attaches a ready/true status", func() {
				_, err := reconciler.Reconcile(ctx, req)
				Expect(err).NotTo(HaveOccurred())

				Expect(repo.GetDeliveryCallCount()).To(Equal(1))

				name := repo.GetDeliveryArgsForCall(0)
				Expect(name).To(Equal("my-new-delivery"))

				Expect(repo.StatusUpdateCallCount()).To(Equal(1))
				deliveryObject, ok := repo.StatusUpdateArgsForCall(0).(*v1alpha1.ClusterDelivery)
				Expect(ok).To(BeTrue())

				Expect(deliveryObject).To(Equal(apiDelivery))
				Expect(deliveryObject.Status.Conditions).To(ContainElements(
					MatchFields(
						IgnoreExtras,
						Fields{
							"Type":   Equal("Ready"),
							"Status": Equal(metav1.ConditionTrue),
							"Reason": Equal("Ready"),
						},
					),
					MatchFields(
						IgnoreExtras,
						Fields{
							"Type":   Equal("TemplatesReady"),
							"Status": Equal(metav1.ConditionTrue),
							"Reason": Equal("Ready"),
						},
					),
				))
			})

			It("updates the status.observedGeneration to equal metadata.generation", func() {
				_, _ = reconciler.Reconcile(ctx, req)

				updatedDelivery := repo.StatusUpdateArgsForCall(0)

				Expect(*updatedDelivery.(*v1alpha1.ClusterDelivery)).To(MatchFields(IgnoreExtras, Fields{
					"Status": MatchFields(IgnoreExtras, Fields{
						"ObservedGeneration": BeEquivalentTo(99),
					}),
				}))
			})

			It("reschedules for 5 seconds", func() {
				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).NotTo(HaveOccurred())

				Expect(result).To(Equal(ctrl.Result{RequeueAfter: 5 * time.Second}))
			})
		})

		Context("a referenced template is not found", func() {
			BeforeEach(func() {
				repo.GetDeliveryClusterTemplateReturnsOnCall(0, nil, nil)
				repo.GetDeliveryClusterTemplateReturnsOnCall(1, nil, errors.New("second-resource not found"))
			})

			It("returns an error without a requeue value", func() {
				_, err := reconciler.Reconcile(ctx, req)
				Expect(err).To(HaveOccurred())

				Expect(err).To(MatchError(ContainSubstring("encountered errors fetching resources: second-resource")))
			})

			It("Sets the status for TemplateNotFound", func() {
				_, err := reconciler.Reconcile(ctx, req)
				Expect(err).To(HaveOccurred())

				Expect(repo.StatusUpdateCallCount()).To(Equal(1))
				deliveryObject, ok := repo.StatusUpdateArgsForCall(0).(*v1alpha1.ClusterDelivery)
				Expect(ok).To(BeTrue())

				Expect(deliveryObject.Status.Conditions).To(ContainElements(
					MatchFields(
						IgnoreExtras,
						Fields{
							"Type":   Equal("Ready"),
							"Status": Equal(metav1.ConditionFalse),
							"Reason": Equal("TemplatesNotFound"),
						},
					),
					MatchFields(
						IgnoreExtras,
						Fields{
							"Type":   Equal("TemplatesReady"),
							"Status": Equal(metav1.ConditionFalse),
							"Reason": Equal("TemplatesNotFound"),
						},
					),
				))
			})

			It("logs all GetTemplate errors encountered", func() {
				_, err := reconciler.Reconcile(ctx, req)
				Expect(err).To(HaveOccurred())

				Expect(out).To(Say(`"msg":"retrieving cluster template"`))
				Expect(out).To(Say(`"error":"second-resource not found"`))
			})
		})

		It("Starts and Finishes cleanly", func() {
			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			Eventually(out).Should(Say(`"msg":"started","name":"my-new-delivery"`))
			Eventually(out).Should(Say(`"msg":"finished","name":"my-new-delivery"`))
		})

	})

	Context("repo.GetDelivery fails", func() {
		It("returns an error", func() {
			repo.GetDeliveryReturns(nil, errors.New("repo.GetDelivery failed"))

			reconciler := delivery.NewReconciler(repo)
			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("get delivery: repo.GetDelivery failed"))
		})
	})

	Context("repo.StatusUpdate fails", func() {
		It("returns an error", func() {
			apiDelivery := &v1alpha1.ClusterDelivery{}
			repo.GetDeliveryReturns(apiDelivery, nil)

			repo.StatusUpdateReturns(errors.New("repo.StatusUpdate failed"))

			reconciler := delivery.NewReconciler(repo)
			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).To(HaveOccurred())

			Expect(err.Error()).To(ContainSubstring("status update: repo.StatusUpdate failed"))
		})
	})

	Context("when the delivery has been deleted from the apiServer", func() {
		BeforeEach(func() {
			repo.GetDeliveryReturns(nil, nil)
		})

		It("does not return an error", func() {
			_, err := reconciler.Reconcile(ctx, req)

			Expect(err).NotTo(HaveOccurred())
		})

		It("does not requeue", func() {
			result, _ := reconciler.Reconcile(ctx, req)

			Expect(result).To(Equal(ctrl.Result{Requeue: false}))
		})
	})
})