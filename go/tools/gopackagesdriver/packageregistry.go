// Copyright 2021 The Bazel Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"strings"
)

type PackageRegistry struct {
	packagesByID map[string]*FlatPackage
	stdlib       map[string]string
}

func NewPackageRegistry(pkgs ...*FlatPackage) *PackageRegistry {
	pr := &PackageRegistry{
		packagesByID: map[string]*FlatPackage{},
		stdlib:       map[string]string{},
	}
	pr.Add(pkgs...)
	return pr
}

func (pr *PackageRegistry) Add(pkgs ...*FlatPackage) *PackageRegistry {
	for _, pkg := range pkgs {
		pr.packagesByID[pkg.ID] = pkg

		if pkg.IsStdlib() {
			pr.stdlib[pkg.PkgPath] = pkg.ID
		}
	}
	return pr
}

func (pr *PackageRegistry) ResolvePaths(prf PathResolverFunc) error {
	for _, pkg := range pr.packagesByID {
		pkg.ResolvePaths(prf)
		pkg.FilterFilesForBuildTags()
	}
	return nil
}

// ResolveImports adds stdlib imports to packages. This is required because
// stdlib packages are not part of the JSON file exports as bazel is unaware of
// them.
func (pr *PackageRegistry) ResolveImports() error {
	resolve := func(importPath string) string {
		if pkgID, ok := pr.stdlib[importPath]; ok {
			return pkgID
		}

		return ""
	}

	for _, pkg := range pr.packagesByID {
		if err := pkg.ResolveImports(resolve); err != nil {
			return err
		}
	}

	return nil
}

func (pr *PackageRegistry) walk(acc map[string]*FlatPackage, root string) {
	pkg := pr.packagesByID[root]

	acc[pkg.ID] = pkg
	for _, pkgID := range pkg.Imports {
		if _, ok := acc[pkgID]; !ok {
			pr.walk(acc, pkgID)
		}
	}
}

func (pr *PackageRegistry) Match(labels []string) ([]string, []*FlatPackage) {
	roots := map[string]struct{}{}

	for _, label := range labels {
		if !strings.HasPrefix(label, "@") {
			label = fmt.Sprintf("@%s", label)
		}

		if label == RulesGoStdlibLabel {
			// For stdlib, we need to append all the subpackages as roots
			// since RulesGoStdLibLabel doesn't actually show up in the stdlib pkg.json
			for _, pkg := range pr.packagesByID {
				if pkg.Standard {
					roots[pkg.ID] = struct{}{}
				}
			}
		} else {
			roots[label] = struct{}{}
		}
	}

	walkedPackages := map[string]*FlatPackage{}
	retRoots := make([]string, 0, len(roots))
	for rootPkg := range roots {
		retRoots = append(retRoots, rootPkg)
		pr.walk(walkedPackages, rootPkg)
	}

	retPkgs := make([]*FlatPackage, 0, len(walkedPackages))
	for _, pkg := range walkedPackages {
		retPkgs = append(retPkgs, pkg)
	}

	return retRoots, retPkgs
}
