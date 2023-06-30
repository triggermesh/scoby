// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resolver

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	resolverFileName    = "/etc/resolv.conf"
	clusterDomainEnvKey = "CLUSTER_DOMAIN"
	defaultDomainName   = "cluster.local"
)

var ClusterDomain string

func init() {
	domain, err := getClusterDomainNameFromResolv()
	if err == nil {
		ClusterDomain = domain
	}

	// Fallback to environment or hardcoded default.
	if ClusterDomain = os.Getenv(clusterDomainEnvKey); len(ClusterDomain) == 0 {
		ClusterDomain = defaultDomainName
	}
}

type Resolver interface {
	ResolveReference(ctx context.Context, ref *v1alpha1.Reference) (*string, error)
	ResolveDestination(ctx context.Context, d *v1alpha1.Destination) (*string, error)
}

func New(client client.Client) Resolver {
	return &resolver{
		client: client,
		domain: ClusterDomain,
	}
}

// Copied from Knative's pkg
func getClusterDomainNameFromResolv() (string, error) {
	f, err := os.Open(resolverFileName)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// First look in the conf file.
	for scanner := bufio.NewScanner(f); scanner.Scan(); {
		elements := strings.Split(scanner.Text(), " ")
		if elements[0] != "search" {
			continue
		}
		for _, e := range elements[1:] {
			if strings.HasPrefix(e, "svc.") {
				return strings.TrimSuffix(e[4:], "."), nil
			}
		}
	}

	return "", fmt.Errorf("could not find the cluster domain at %q", resolverFileName)
}

type resolver struct {
	client client.Client
	domain string
}

func (r *resolver) ResolveReference(ctx context.Context, ref *v1alpha1.Reference) (*string, error) {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(ref.APIVersion)
	u.SetKind(ref.Kind)
	u.SetNamespace(ref.Namespace)
	u.SetName(ref.Name)

	if err := r.client.Get(ctx, client.ObjectKeyFromObject(u), u); err != nil {
		return nil, err
	}

	// K8s Services are special cased. They can be called, even though they do not satisfy the
	// Callable interface.
	if ref.APIVersion == "v1" && ref.Kind == "Service" {
		u := fmt.Sprintf("http://%s.%s.svc.%s", ref.Name, ref.Namespace, r.domain)
		return &u, nil
	}

	url, b, err := unstructured.NestedString(u.Object, "status", "address", "url")
	switch {
	case err != nil:
		return nil, fmt.Errorf(`unexpected value at "status.address.url": %+v`, err)
	case !b || url == "":
		return nil, errors.New(`object does not inform "status.address.url"`)
	}

	return &url, nil
}

func (r *resolver) ResolveDestination(ctx context.Context, d *v1alpha1.Destination) (*string, error) {
	if d.Ref == nil && d.URI == nil {
		return nil, errors.New("ref or uri should be informed")
	}

	u := ""
	if d.Ref != nil {
		uri, err := r.ResolveReference(ctx, d.Ref)
		if err != nil {
			return nil, err
		}

		parsed, err := url.Parse(*uri)
		if err != nil {
			return nil, fmt.Errorf("Resolved reference to %q cannot be parsed: %w", *uri, err)
		}

		if d.URI != nil {
			host, port := d.URI.URL().Hostname(), d.URI.URL().Port()

			if host != "" {
				return nil, fmt.Errorf("URI should not contain a host while also informing a Reference: %q", d.URI.String())
			}

			if port != "" {
				parsed.Host = parsed.Hostname() + ":" + port
			}

			if d.URI.Scheme != "" {
				parsed.Scheme = d.URI.Scheme
			}

			if d.URI.RawQuery != "" {
				parsed.RawQuery = d.URI.RawQuery
			}

			if d.URI.Path != "" {
				parsed.Path = d.URI.Path
			}
		}

		u = parsed.String()

	} else {
		parsed, err := url.Parse(d.URI.String())
		if err != nil {
			return nil, fmt.Errorf("URI cannot be parsed: %w", err)
		}
		u = parsed.String()
	}

	return &u, nil
}
