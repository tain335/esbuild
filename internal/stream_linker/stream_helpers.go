package streamlinker

import (
	"github.com/evanw/esbuild/internal/config"
	"github.com/evanw/esbuild/internal/js_ast"
	"github.com/evanw/esbuild/internal/js_printer"
	"github.com/evanw/esbuild/internal/logger"
	"github.com/evanw/esbuild/internal/renamer"
	"github.com/evanw/esbuild/internal/sourcemap"
)

func combineParts(ast *js_ast.AST) *js_ast.AST {
	mainPart := ast.Parts[0]
	for i := 1; i < len(ast.Parts); i++ {
		mainPart.Stmts = append(mainPart.Stmts, ast.Parts[i].Stmts...)
		mainPart.Scopes = append(mainPart.Scopes, ast.Parts[i].Scopes...)
		mainPart.Dependencies = append(mainPart.Dependencies, ast.Parts[i].Dependencies...)
		mainPart.ImportRecordIndices = append(mainPart.ImportRecordIndices, ast.Parts[i].ImportRecordIndices...)
		mainPart.DeclaredSymbols = append(mainPart.DeclaredSymbols, ast.Parts[i].DeclaredSymbols...)
		for ref, val := range ast.Parts[i].SymbolUses {
			mainPart.SymbolUses[ref] = val
		}
		if mainPart.SymbolCallUses == nil {
			mainPart.SymbolCallUses = make(map[js_ast.Ref]js_ast.SymbolCallUse)
		}
		for ref, val := range ast.Parts[i].SymbolCallUses {
			mainPart.SymbolCallUses[ref] = val
		}
	}
	ast.Parts = []js_ast.Part{mainPart}
	return ast
}

func generateNewSymbol(symbols *js_ast.SymbolMap, ast *js_ast.AST, kind js_ast.SymbolKind, originalName string) js_ast.Ref {
	var sourceSymbols *[]js_ast.Symbol
	if symbols, ok := symbols.SymbolsForSource[ast.ModuleRef.SourceIndex]; ok {
		sourceSymbols = &symbols
	}
	ref := js_ast.Ref{
		SourceIndex: ast.ModuleRef.SourceIndex,
		InnerIndex:  uint32(len(*sourceSymbols)),
	}

	ast.Symbols = append(*sourceSymbols, js_ast.Symbol{
		Kind:         kind,
		OriginalName: originalName,
		Link:         js_ast.InvalidRef,
	})
	symbols.SymbolsForSource[ast.ModuleRef.SourceIndex] = ast.Symbols

	ast.ModuleScope.Generated = append(ast.ModuleScope.Generated, ref)
	return ref
}

func printJS(source logger.Source, ast *js_ast.AST, symbols *js_ast.SymbolMap, inputSourceMap *sourcemap.SourceMap, options *config.Options) js_printer.PrintResult {
	r := renamer.NewNoOpRenamer(*symbols)
	lineOffsetTable := sourcemap.GenerateLineOffsetTables(source.Contents, ast.ApproximateLineCount)
	return js_printer.Print(*ast, *symbols, r, js_printer.Options{
		LineOffsetTables:    lineOffsetTable,
		AddSourceMappings:   options.SourceMap != config.SourceMapNone,
		SourceMap:           options.SourceMap,
		OutputFormat:        config.FormatCommonJS,
		InputSourceMap:      inputSourceMap,
		UnsupportedFeatures: options.UnsupportedJSFeatureOverrides,
		RequireOrImportMetaForSource: func(u uint32) (meta js_printer.RequireOrImportMeta) {
			return
		},
	})
}
