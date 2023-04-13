// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath"
	"gopkg.in/yaml.v3"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func setupLog(msg string) env.Func {
	log.Printf("\033[32m--- %s\033[0m", msg)
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		return ctx, nil
	}
}

func getYAMLField(filename, fieldpath string) string {
	data, err := os.ReadFile(filepath.Join("./testdata", filename))
	if err != nil {
		return ""
	}

	var n yaml.Node

	if err := yaml.Unmarshal(data, &n); err != nil {
		log.Fatalf("cannot unmarshal data: %v", err)
	}

	p, err := yamlpath.NewPath(fieldpath)
	if err != nil {
		log.Fatalf("cannot create path: %v", err)
	}

	q, err := p.Find(&n)
	if err != nil {
		log.Fatalf("unexpected error: %v", err)
	}

	if len(q) != 1 {
		log.Fatal("multiple matches for field path")
	}

	return q[0].Value
}
