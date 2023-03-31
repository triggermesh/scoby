//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CRDRegistration) DeepCopyInto(out *CRDRegistration) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CRDRegistration.
func (in *CRDRegistration) DeepCopy() *CRDRegistration {
	if in == nil {
		return nil
	}
	out := new(CRDRegistration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CRDRegistration) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CRDRegistrationList) DeepCopyInto(out *CRDRegistrationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CRDRegistration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CRDRegistrationList.
func (in *CRDRegistrationList) DeepCopy() *CRDRegistrationList {
	if in == nil {
		return nil
	}
	out := new(CRDRegistrationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CRDRegistrationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CRDRegistrationSpec) DeepCopyInto(out *CRDRegistrationSpec) {
	*out = *in
	in.Workload.DeepCopyInto(&out.Workload)
	if in.Hook != nil {
		in, out := &in.Hook, &out.Hook
		*out = new(commonv1alpha1.Hook)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CRDRegistrationSpec.
func (in *CRDRegistrationSpec) DeepCopy() *CRDRegistrationSpec {
	if in == nil {
		return nil
	}
	out := new(CRDRegistrationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CRDRegistrationStatus) DeepCopyInto(out *CRDRegistrationStatus) {
	*out = *in
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CRDRegistrationStatus.
func (in *CRDRegistrationStatus) DeepCopy() *CRDRegistrationStatus {
	if in == nil {
		return nil
	}
	out := new(CRDRegistrationStatus)
	in.DeepCopyInto(out)
	return out
}
