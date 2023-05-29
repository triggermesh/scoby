// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"
	"time"

	"go.uber.org/automaxprocs/maxprocs"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	servingv1 "knative.dev/serving/pkg/apis/serving/v1"

	scobyv1alpha1 "github.com/triggermesh/scoby/pkg/apis/scoby/v1alpha1"
	crbuilder "github.com/triggermesh/scoby/pkg/component/builder"
	scobyconfig "github.com/triggermesh/scoby/pkg/config"
	"github.com/triggermesh/scoby/pkg/registration/reconciler/crd"
	"github.com/triggermesh/scoby/pkg/registration/registry"
	"github.com/triggermesh/scoby/pkg/utils/configmap"
	"github.com/triggermesh/scoby/pkg/utils/resolver"
)

const (
	registryGracefulTimeout = 5 * time.Second
)

func main() {
	// Parse configuration from environment variables.
	scobyconfig.ParseFromEnvironment()

	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	log := ctrl.Log

	undo, err := maxprocs.Set()
	if err != nil {
		log.Error(err, "could not match available CPUs to processes")
		os.Exit(1)
	}
	defer undo()

	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to find kubernetes config")
		os.Exit(1)
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
	})
	if err != nil {
		log.Error(err, "unable to create controller manager")
		os.Exit(1)
	}
	log.V(1).Info("controller manager created")

	if err := scobyv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "could not add scoby API to scheme")
		os.Exit(1)
	}

	if err := apiextensionsv1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "could not add apiextensions API to scheme")
		os.Exit(1)
	}

	// Although added to scheme, Knative Serving is a rendering option and
	// there is no runtime requirement for Knative to be installed.
	if err := servingv1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "could not add knative serving API to scheme")
		os.Exit(1)
	}

	// The resolver object performs Kubernetes Object resolution
	// into URL.
	reslv := resolver.New(mgr.GetClient())

	// ConfigMap reader at the Scoby controller namespace will help
	// reading ConfigMap contents and using them for rendering user workloads.
	cmr := configmap.NewNamespacedReader(scobyconfig.Get().ScobyNamespace(), mgr.GetClient())

	// Builder for component reconcilers
	crb := crbuilder.NewBuilder(mgr, reslv, cmr)

	// Parent context.
	ctx := ctrl.SetupSignalHandler()

	cl := log.WithName("component")
	reg := registry.New(ctx, crb, &cl)

	r := crd.New(mgr.GetClient(), reg, reslv, cl.WithName("crdregistration"))

	if err := builder.ControllerManagedBy(mgr).
		For(&scobyv1alpha1.CRDRegistration{}).
		Complete(r); err != nil {
		log.Error(err, "could not build controller for CRD registration")
		os.Exit(1)

	}

	// TODO setup metrics
	// TODO setup profiler

	// Start manager
	if err := mgr.Start(ctx); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}

	// Make sure registered controllers exit as cleanly as possible
	select {
	case err := <-reg.WaitStopChannel():
		if err != nil {
			cl.Error(err, "registered controllers did not shut down gracefully")
		}
	case <-time.After(registryGracefulTimeout):
		cl.Error(err, "registered controllers stop timed out", "timeout", registryGracefulTimeout)
	}
}
