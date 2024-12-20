// Package main provides a custom multichecker tool for Go static analysis.
// This tool integrates:
//  1. Standard analyzers from golang.org/x/tools/go/analysis/passes
//  2. All SA (Staticcheck) analyzers from staticcheck.io
//  3. ST1000 analyzer from Stylecheck (checks package comments and their style)
//  4. Additional public analyzers:
//     errcheck (ensures errors are checked),
//     bodyclose (checks HTTP response bodies are closed)
//  5. A custom ExitCheckAnalyzer that prohibits direct calls to os.Exit in the main function of the main package.
//
// Usage:
//
// To run the multichecker on your code:
//
//	go run ./cmd/staticlint [packages...]
//
// Example:
//
//	go run ./cmd/staticlint ./cmd/shortener
package main

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"

	"golang.org/x/tools/go/analysis/passes/appends"
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/atomicalign"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/defers"
	"golang.org/x/tools/go/analysis/passes/directive"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/fieldalignment"
	"golang.org/x/tools/go/analysis/passes/findcall"
	"golang.org/x/tools/go/analysis/passes/framepointer"
	"golang.org/x/tools/go/analysis/passes/httpmux"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/pkgfact"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/reflectvaluecompare"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/sigchanyzer"
	"golang.org/x/tools/go/analysis/passes/slog"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stdversion"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/testinggoroutine"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/timeformat"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"golang.org/x/tools/go/analysis/passes/unusedwrite"
	"golang.org/x/tools/go/analysis/passes/usesgenerics"
	"golang.org/x/tools/go/analysis/passes/waitgroup"

	"honnef.co/go/tools/analysis/facts/nilness"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"

	"github.com/kisielk/errcheck/errcheck"
	"github.com/timakin/bodyclose/passes/bodyclose"
)

// ExitCheckAnalyzer is a custom analyzer that prohibits calling os.Exit in the main function of package main.
var ExitCheckAnalyzer = &analysis.Analyzer{
	Name: "exitcheck",
	Doc:  "reports calls to os.Exit in main function of main package",
	Run:  runExitCheckAnalyzer,
}

func runExitCheckAnalyzer(pass *analysis.Pass) (interface{}, error) {
	if pass.Pkg.Name() != "main" {
		// Not the main package, no checks needed.
		return nil, nil
	}

	// Avoid reports of calls to os.Exit in test files.
	for _, f := range pass.Files {
		// Check all comments of the file.
		for _, comm := range f.Comments {
			for _, c := range comm.List {
				// Skip analysis if found code generated by 'go test'.
				if strings.Contains(c.Text, "Code generated by 'go test'. DO NOT EDIT.") {
					return nil, nil
				}
			}
		}
	}

	var mainFunc *ast.FuncDecl
	// Find main function.
	for _, f := range pass.Files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Name.Name == "main" && fn.Recv == nil {
				mainFunc = fn
				break
			}
		}
	}

	// Skip analysis if main function not found.
	if mainFunc == nil {
		return nil, nil
	}

	// Inspect the main function for calls to os.Exit.
	ast.Inspect(mainFunc.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check for call to os.Exit.
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "os" && fun.Sel.Name == "Exit" {
				pass.Reportf(call.Pos(), "calling os.Exit in main function of main package is prohibited")
			}
		}

		return true
	})

	return nil, nil
}

func main() {
	// Collect analyzers.
	var analyzers []*analysis.Analyzer

	// Standard analyzers from golang.org/x/tools/go/analysis/passes
	stdAnalyzers := []*analysis.Analyzer{
		appends.Analyzer,
		asmdecl.Analyzer,
		assign.Analyzer,
		atomic.Analyzer,
		atomicalign.Analyzer,
		bools.Analyzer,
		buildssa.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		ctrlflow.Analyzer,
		deepequalerrors.Analyzer,
		defers.Analyzer,
		directive.Analyzer,
		errorsas.Analyzer,
		fieldalignment.Analyzer,
		findcall.Analyzer,
		framepointer.Analyzer,
		httpmux.Analyzer,
		httpresponse.Analyzer,
		ifaceassert.Analyzer,
		inspect.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		nilness.Analysis,
		pkgfact.Analyzer,
		printf.Analyzer,
		reflectvaluecompare.Analyzer,
		shadow.Analyzer,
		shift.Analyzer,
		sigchanyzer.Analyzer,
		slog.Analyzer,
		sortslice.Analyzer,
		stdmethods.Analyzer,
		stdversion.Analyzer,
		stringintconv.Analyzer,
		structtag.Analyzer,
		testinggoroutine.Analyzer,
		tests.Analyzer,
		timeformat.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		unsafeptr.Analyzer,
		unusedresult.Analyzer,
		unusedwrite.Analyzer,
		usesgenerics.Analyzer,
		waitgroup.Analyzer,
	}
	analyzers = append(analyzers, stdAnalyzers...)

	// Add all SA analyzers (Staticcheck).
	// staticcheck.Analyzers is a slice of struct {Analyzer *analysis.Analyzer}.
	for _, a := range staticcheck.Analyzers {
		if strings.HasPrefix(a.Analyzer.Name, "SA") {
			analyzers = append(analyzers, a.Analyzer)
		}
	}

	// Add ST1000 (Stylecheck) analyzer.
	// stylecheck.Analyzers is a slice of struct {Analyzer *analysis.Analyzer}.
	for _, a := range stylecheck.Analyzers {
		if a.Analyzer.Name == "ST1000" {
			analyzers = append(analyzers, a.Analyzer)
			break
		}
	}

	// Add errcheck and bodyclose analyzers.
	analyzers = append(analyzers, errcheck.Analyzer)
	analyzers = append(analyzers, bodyclose.Analyzer)

	// 5. Add our custom analyzer.
	analyzers = append(analyzers, ExitCheckAnalyzer)

	// Run multichecker.
	multichecker.Main(analyzers...)
}