// Copyright 2016-2020, Pulumi Corporation.
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

package pcl

import (
	"fmt"
	"io"
	"sort"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

// Node represents a single definition in a program or component. Nodes may be config, locals, resources, or outputs.
type Node interface {
	model.Definition

	// Name returns the lexical name of the node.
	Name() string

	// Type returns the type of the node.
	Type() model.Type

	// VisitExpressions visits the expressions that make up the node's body.
	VisitExpressions(pre, post model.ExpressionVisitor) hcl.Diagnostics

	markBinding()
	markBound()
	isBinding() bool
	isBound() bool

	getDependencies() []Node
	setDependencies(nodes []Node)

	isNode()
}

type node struct {
	binding bool
	bound   bool
	deps    []Node
}

func (r *node) markBinding() {
	r.binding = true
}

func (r *node) markBound() {
	r.bound = true
}

func (r *node) isBinding() bool {
	return r.binding && !r.bound
}

func (r *node) isBound() bool {
	return r.bound
}

func (r *node) getDependencies() []Node {
	return r.deps
}

func (r *node) setDependencies(nodes []Node) {
	r.deps = nodes
}

func (*node) isNode() {}

// Program represents a semantically-analyzed Pulumi HCL2 program.
type Program struct {
	Nodes []Node

	files []*syntax.File

	binder *binder
}

// NewDiagnosticWriter creates a new hcl.DiagnosticWriter for use with diagnostics generated by the program.
func (p *Program) NewDiagnosticWriter(w io.Writer, width uint, color bool) hcl.DiagnosticWriter {
	return syntax.NewDiagnosticWriter(w, p.files, width, color)
}

// BindExpression binds an HCL2 expression in the top-level context of the program.
func (p *Program) BindExpression(node hclsyntax.Node) (model.Expression, hcl.Diagnostics) {
	return p.binder.bindExpression(node)
}

// Packages returns the list of package referenced used by this program.
func (p *Program) Packages() []*schema.Package {
	refs := p.PackageReferences()
	defs := make([]*schema.Package, len(refs))
	for i, ref := range refs {
		def, err := ref.Definition()
		if err != nil {
			panic(fmt.Errorf("loading package definition: %w", err))
		}
		defs[i] = def
	}
	return defs
}

// PackageReferences returns the list of package referenced used by this program.
func (p *Program) PackageReferences() []schema.PackageReference {
	keys := make([]string, 0, len(p.binder.referencedPackages))
	for k := range p.binder.referencedPackages {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	values := make([]schema.PackageReference, 0, len(p.binder.referencedPackages))
	for _, k := range keys {
		values = append(values, p.binder.referencedPackages[k])
	}
	return values
}

// PackageSnapshots returns the list of packages schemas used by this program. If a referenced package is partial,
// its returned value is a snapshot that contains only the package members referenced by the program. Otherwise, its
// returned value is the full package definition.
func (p *Program) PackageSnapshots() ([]*schema.Package, error) {
	keys := make([]string, 0, len(p.binder.referencedPackages))
	for k := range p.binder.referencedPackages {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	values := make([]*schema.Package, 0, len(p.binder.referencedPackages))
	for _, k := range keys {
		ref := p.binder.referencedPackages[k]

		var pkg *schema.Package
		var err error
		if partial, ok := ref.(*schema.PartialPackage); ok {
			pkg, err = partial.Snapshot()
		} else {
			pkg, err = ref.Definition()
		}
		if err != nil {
			return nil, fmt.Errorf("defining package '%v': %w", ref.Name(), err)
		}

		values = append(values, pkg)
	}
	return values, nil
}
