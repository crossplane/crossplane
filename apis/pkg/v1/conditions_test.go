package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"

	commonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

func TestPackageHealth(t *testing.T) {
	type args struct {
		pr PackageRevision
	}
	type want struct {
		condition commonv1.Condition
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"HealthyRevisionAndRuntimeWithRuntime": {
			reason: "Should return healthy condition when both revision and runtime are healthy for package with runtime",
			args: args{
				pr: &ProviderRevision{
					Status: PackageRevisionStatus{
						ConditionedStatus: commonv1.ConditionedStatus{
							Conditions: []commonv1.Condition{
								{
									Type:    TypeRevisionHealthy,
									Status:  corev1.ConditionTrue,
									Reason:  ReasonHealthy,
									Message: "Package revision is healthy",
								},
								{
									Type:    TypeRuntimeHealthy,
									Status:  corev1.ConditionTrue,
									Reason:  ReasonHealthy,
									Message: "Package runtime is healthy",
								},
							},
						},
					},
				},
			},
			want: want{
				condition: Healthy(),
			},
		},
		"HealthyRevisionWithoutRuntime": {
			reason: "Should return healthy condition when revision is healthy for package without runtime",
			args: args{
				pr: &ConfigurationRevision{
					Status: PackageRevisionStatus{
						ConditionedStatus: commonv1.ConditionedStatus{
							Conditions: []commonv1.Condition{
								{
									Type:    TypeRevisionHealthy,
									Status:  corev1.ConditionTrue,
									Reason:  ReasonHealthy,
									Message: "Package revision is healthy",
								},
							},
						},
					},
				},
			},
			want: want{
				condition: Healthy(),
			},
		},
		"UnhealthyRevision": {
			reason: "Should return unhealthy condition when revision is unhealthy",
			args: args{
				pr: &ProviderRevision{
					Status: PackageRevisionStatus{
						ConditionedStatus: commonv1.ConditionedStatus{
							Conditions: []commonv1.Condition{
								{
									Type:    TypeRevisionHealthy,
									Status:  corev1.ConditionFalse,
									Reason:  ReasonUnhealthy,
									Message: "Package revision is not ready",
								},
								{
									Type:    TypeRuntimeHealthy,
									Status:  corev1.ConditionTrue,
									Reason:  ReasonHealthy,
									Message: "Package runtime is healthy",
								},
							},
						},
					},
				},
			},
			want: want{
				condition: Unhealthy().WithMessage("Package revision health is \"False\" with message: Package revision is not ready"),
			},
		},
		"UnhealthyRuntime": {
			reason: "Should return unhealthy condition when runtime is unhealthy",
			args: args{
				pr: &ProviderRevision{
					Status: PackageRevisionStatus{
						ConditionedStatus: commonv1.ConditionedStatus{
							Conditions: []commonv1.Condition{
								{
									Type:    TypeRevisionHealthy,
									Status:  corev1.ConditionTrue,
									Reason:  ReasonHealthy,
									Message: "Package revision is healthy",
								},
								{
									Type:    TypeRuntimeHealthy,
									Status:  corev1.ConditionFalse,
									Reason:  ReasonUnhealthy,
									Message: "Runtime deployment is not ready",
								},
							},
						},
					},
				},
			},
			want: want{
				condition: Unhealthy().WithMessage("Package runtime health is \"False\" with message: Runtime deployment is not ready"),
			},
		},
		"BothUnhealthy": {
			reason: "Should return unhealthy condition with revision message when both are unhealthy (revision checked first)",
			args: args{
				pr: &FunctionRevision{
					Status: FunctionRevisionStatus{
						PackageRevisionStatus: PackageRevisionStatus{
							ConditionedStatus: commonv1.ConditionedStatus{
								Conditions: []commonv1.Condition{
									{
										Type:    TypeRevisionHealthy,
										Status:  corev1.ConditionFalse,
										Reason:  ReasonUnhealthy,
										Message: "Package revision is not ready",
									},
									{
										Type:    TypeRuntimeHealthy,
										Status:  corev1.ConditionFalse,
										Reason:  ReasonUnhealthy,
										Message: "Runtime deployment is not ready",
									},
								},
							},
						},
					},
				},
			},
			want: want{
				condition: Unhealthy().WithMessage("Package revision health is \"False\" with message: Package revision is not ready"),
			},
		},
		"UnknownRevisionHealth": {
			reason: "Should return unhealthy condition when revision health is unknown",
			args: args{
				pr: &ProviderRevision{
					Status: PackageRevisionStatus{
						ConditionedStatus: commonv1.ConditionedStatus{
							Conditions: []commonv1.Condition{
								{
									Type:   TypeRevisionHealthy,
									Status: corev1.ConditionUnknown,
									Reason: ReasonUnknownHealth,
								},
								{
									Type:    TypeRuntimeHealthy,
									Status:  corev1.ConditionTrue,
									Reason:  ReasonHealthy,
									Message: "Package runtime is healthy",
								},
							},
						},
					},
				},
			},
			want: want{
				condition: Unhealthy().WithMessage("Package revision health is \"Unknown\""),
			},
		},
		"UnknownRuntimeHealth": {
			reason: "Should return unhealthy condition when runtime health is unknown for package with runtime",
			args: args{
				pr: &ProviderRevision{
					Status: PackageRevisionStatus{
						ConditionedStatus: commonv1.ConditionedStatus{
							Conditions: []commonv1.Condition{
								{
									Type:    TypeRevisionHealthy,
									Status:  corev1.ConditionTrue,
									Reason:  ReasonHealthy,
									Message: "Package revision is healthy",
								},
								{
									Type:   TypeRuntimeHealthy,
									Status: corev1.ConditionUnknown,
									Reason: ReasonUnknownHealth,
								},
							},
						},
					},
				},
			},
			want: want{
				condition: Unhealthy().WithMessage("Package runtime health is \"Unknown\""),
			},
		},
		"MissingConditions": {
			reason: "Should return unhealthy condition when conditions are missing",
			args: args{
				pr: &ConfigurationRevision{
					Status: PackageRevisionStatus{
						ConditionedStatus: commonv1.ConditionedStatus{
							Conditions: []commonv1.Condition{},
						},
					},
				},
			},
			want: want{
				condition: Unhealthy().WithMessage("Package revision health is \"Unknown\""),
			},
		},
		"FunctionRevisionHealthy": {
			reason: "Should return healthy condition for function revision with both conditions healthy",
			args: args{
				pr: &FunctionRevision{
					Status: FunctionRevisionStatus{
						PackageRevisionStatus: PackageRevisionStatus{
							ConditionedStatus: commonv1.ConditionedStatus{
								Conditions: []commonv1.Condition{
									{
										Type:    TypeRevisionHealthy,
										Status:  corev1.ConditionTrue,
										Reason:  ReasonHealthy,
										Message: "Function revision is healthy",
									},
									{
										Type:    TypeRuntimeHealthy,
										Status:  corev1.ConditionTrue,
										Reason:  ReasonHealthy,
										Message: "Function runtime is healthy",
									},
								},
							},
						},
					},
				},
			},
			want: want{
				condition: Healthy(),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := PackageHealth(tc.args.pr)

			if diff := cmp.Diff(tc.want.condition, got); diff != "" {
				t.Errorf("\n%s\npackageHealth(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
