// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/triggermesh/scoby/pkg/apis/scoby.triggermesh.io/v1alpha1"
	"github.com/triggermesh/scoby/pkg/reconciler/component"
	"github.com/triggermesh/scoby/pkg/reconciler/registration/base"
	"github.com/triggermesh/scoby/pkg/reconciler/registration/crd"
	genreg "github.com/triggermesh/scoby/pkg/reconciler/registration/generic"
)

// var log = logf.Log.WithName("scoby")

func main() {
	opts := zap.Options{
		Development: false,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	log := ctrl.Log

	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "Unable to find kubernetes config")
		os.Exit(1)
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	mgr, err := manager.New(cfg, manager.Options{
		Scheme: scheme,
	})
	if err != nil {
		log.Error(err, "Unable to create controller manager")
		os.Exit(1)
	}
	log.V(1).Info("Controller manager created")

	if err := v1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "could not add scoby API to scheme")
		os.Exit(1)
	}

	if err := apiextensionsv1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "could not add apiextensions API to scheme")
		os.Exit(1)
	}

	// Parent context.
	ctx := signals.SetupSignalHandler()

	// Create base reconciler
	bl := log.WithName("regbase")
	br := base.New(mgr.GetClient(), &bl)

	cl := log.WithName("component")
	reg := component.NewControllerRegistry(ctx, mgr, &cl)

	r := &crd.Reconciler{
		Registry: reg,
	}
	if err := builder.ControllerManagedBy(mgr).
		For(&v1alpha1.CRDRegistration{}).
		Complete(r); err != nil {
		log.Error(err, "could not build controller for CRD registration")
		os.Exit(1)

	}

	// Setup generic reconciler
	err = genreg.SetupReconciler(mgr, br)
	if err != nil {
		log.Error(err, "Unable to setup registration reconciler")
		os.Exit(1)
	}

	// Start manager
	if err := mgr.Start(ctx); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}

	// TODO setup metrics
	// TODO setup profiler
}
