package packlinker

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/evanw/esbuild/internal/ast"
	"github.com/evanw/esbuild/internal/bundler"
	"github.com/evanw/esbuild/internal/config"
	"github.com/evanw/esbuild/internal/fs"
	"github.com/evanw/esbuild/internal/graph"
	"github.com/evanw/esbuild/internal/helpers"
	"github.com/evanw/esbuild/internal/hot"
	"github.com/evanw/esbuild/internal/js_ast"
	"github.com/evanw/esbuild/internal/js_parser"
	"github.com/evanw/esbuild/internal/js_printer"
	"github.com/evanw/esbuild/internal/js_template"
	"github.com/evanw/esbuild/internal/logger"
	"github.com/evanw/esbuild/internal/renamer"
	"github.com/evanw/esbuild/internal/resolver"
	"github.com/evanw/esbuild/internal/runtime"
)

// TODO
// 1. 统一到PackLinkContext
// 2. 并行转换
// 3. 梳理代码

// 记录全局的ImportRecord，因为所有js的ast要合并成一个ast，所以所有的importRecords都要合并在一起
var importRecordIndexSlice = []*uint32{}
var originImportRecordIndex = []uint32{}

var visitedMap = make(map[uint32]bool)

var headStmts = make([]js_ast.Stmt, 0, 64)
var tailStmts = make([]js_ast.Stmt, 0, 64)
var isESModule = false
var shouldRemoveNextExportStmt = false

// cache
var packModuleCache = make(map[string]PackModule)
var chunkPackModules = make(map[string]map[string]PackModule)
var newChunkPackModules = make(map[string]map[string]PackModule)

var currentHash string

var reactComponentOrHooks = make(map[logger.Loc]*ReactSignature)

type ReactSignature struct {
	binding     js_ast.Ref
	hooks       []js_ast.Expr
	customHooks []js_ast.Expr
	fnBody      *js_ast.FnBody
	isHook      bool
}

type PackModule struct {
	Source                  logger.Source
	AST                     *js_ast.AST
	OriginImportRecordIndex []uint32
	ImportRecordIndexSlice  []*uint32
	OriginModule            *graph.JSRepr
}

func addImportRecordIndex(index *uint32) {
	importRecordIndexSlice = append(importRecordIndexSlice, index)
	originImportRecordIndex = append(originImportRecordIndex, *index)
}

func addStmtToFunc(packFunctionAST *js_ast.AST, stmts []js_ast.Stmt, tail bool) {
	funcStatement := packFunctionAST.Parts[1].Stmts[0]
	funcBody := &funcStatement.Data.(*js_ast.SExpr).Value.Data.(*js_ast.EFunction).Fn.Body
	if tail {
		funcBody.Block.Stmts = append(funcBody.Block.Stmts, stmts...)
	} else {
		funcBody.Block.Stmts = append(stmts, funcBody.Block.Stmts...)
	}
}

func generateNewSymbol(g *graph.LinkerGraph, ast *js_ast.AST, kind js_ast.SymbolKind, originalName string) js_ast.Ref {
	var sourceSymbols *[]js_ast.Symbol
	if symbols, ok := g.Symbols.SymbolsForSource[ast.ModuleRef.SourceIndex]; ok {
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
	g.Symbols.SymbolsForSource[ast.ModuleRef.SourceIndex] = ast.Symbols
	return ref
}

func generateCJS(log logger.Log, g *graph.LinkerGraph, module *graph.JSRepr) *js_ast.AST {
	moduleRef := generateNewSymbol(g, &module.AST, js_ast.SymbolHoisted, "module")
	exportsRef := generateNewSymbol(g, &module.AST, js_ast.SymbolHoisted, "exports")
	requireRef := generateNewSymbol(g, &module.AST, js_ast.SymbolHoisted, "require")

	if len(module.AST.Parts) == 1 {
		newPart := js_ast.Part{
			Stmts: []js_ast.Stmt{},
		}
		module.AST.Parts = append(module.AST.Parts, newPart)
	}
	part := &module.AST.Parts[1]
	part.Stmts = []js_ast.Stmt{
		{
			Data: &js_ast.SExpr{
				Value: js_ast.Expr{
					Data: &js_ast.EFunction{
						Fn: js_ast.Fn{
							Args: []js_ast.Arg{
								{
									Binding: js_ast.Binding{
										Data: &js_ast.BIdentifier{
											Ref: moduleRef,
										},
									},
								},
								{
									Binding: js_ast.Binding{
										Data: &js_ast.BIdentifier{
											Ref: exportsRef,
										},
									},
								},
								{
									Binding: js_ast.Binding{
										Data: &js_ast.BIdentifier{
											Ref: requireRef,
										},
									},
								},
							},
							Body: js_ast.FnBody{
								Block: js_ast.SBlock{
									Stmts: part.Stmts,
								},
							},
						},
					},
				},
			},
		},
	}
	// TODO 可以移除
	module.AST.Symbols = g.Symbols.SymbolsForSource[module.AST.ModuleRef.SourceIndex]
	return &module.AST
}

func generateHotUpdateTemplate(log logger.Log, chunkId string) *js_ast.AST {
	return js_template.Template(log, js_ast.MakeDynamicIndex(), "webpackHotUpdate($$chunkId$$, {})", map[string]interface{}{"chunkId": &js_ast.EString{Value: helpers.StringToUTF16(chunkId)}}, js_parser.Options{})
}

func generateCSSModule(log logger.Log, g *graph.LinkerGraph, source logger.Source) *graph.JSRepr {
	ast := js_template.Template(log, js_ast.MakeDynamicIndex(), `import api from "<style_runtime>";
	var content = $$content$$;
	if (typeof content === 'string') {
		content = [[module.i, content, '']];
	}
	var options = {};
	options.insert = "head";
	options.singleton = false;
	var update = api(content, options);
	module.hot.accept();
	module.hot.dispose(function() {
		update();
	});`, map[string]interface{}{"content": &js_ast.EString{Value: helpers.StringToUTF16(source.Contents)}}, js_parser.Options{})
	g.Symbols.SymbolsForSource[ast.ModuleRef.SourceIndex] = ast.Symbols
	return &graph.JSRepr{
		AST: *ast,
	}
}

func transformCSSToJS(log logger.Log, g *graph.LinkerGraph, module *graph.JSRepr) *graph.JSRepr {
	return generateCSSModule(log, g, g.Files[module.CSSSourceIndex.GetIndex()].InputFile.Source)
}

func transformOtherToJS(module *graph.JSRepr) *graph.JSRepr {
	part := module.AST.Parts[1]
	lazy, ok := part.Stmts[0].Data.(*js_ast.SLazyExport)
	if !ok {
		panic("Internal error")
	}
	part.Stmts = []js_ast.Stmt{js_ast.AssignStmt(
		js_ast.Expr{Loc: lazy.Value.Loc, Data: &js_ast.EDot{
			Target:  js_ast.Expr{Loc: lazy.Value.Loc, Data: &js_ast.EIdentifier{Ref: module.AST.ModuleRef}},
			Name:    "exports",
			NameLoc: lazy.Value.Loc,
		}},
		lazy.Value,
	)}
	module.AST.HasLazyExport = false
	return module
}

func computeReactSignautreKey(g *graph.LinkerGraph, ast *js_ast.AST, signature ReactSignature) string {
	key := ""
	for _, hook := range signature.hooks {
		newAst := *ast
		newAst.Parts = []js_ast.Part{
			{
				Stmts: []js_ast.Stmt{{
					Data: &js_ast.SExpr{
						Value: hook,
					},
				}},
			},
		}
		key += string(printJS(&newAst, g).JS)
	}
	hash := md5.Sum([]byte(key))
	md5str := fmt.Sprintf("%x", hash)
	return md5str
}

/**
如何处理这种情况？
function App() {
	useCustomHook();
	function useCustomHook() {
		useState(0);
	}
}
不处理 webpack-react-refresh-plugin 也没处理
*/
func addReactHooks(g *graph.LinkerGraph, ast *js_ast.AST, nodePath *js_ast.NodePath, name string, e js_ast.Expr, target js_ast.Expr) {
	for {
		if nodePath.Node != nil {
			node := nodePath.Node
			switch node.(type) {
			case *js_ast.Stmt:
				if f, ok := node.(*js_ast.Stmt).Data.(*js_ast.SFunction); ok {
					if signature, ok := reactComponentOrHooks[f.Fn.Body.Loc]; ok {
						signature.hooks = append(signature.hooks, e)
						if !isReactBuiltInHooks(name) {
							signature.customHooks = append(signature.customHooks, target)
						}
						signature.fnBody = &f.Fn.Body
						return
					}
				}
			case *js_ast.Expr:
				switch node.(*js_ast.Expr).Data.(type) {
				case *js_ast.EFunction, *js_ast.EArrow:
					if f, ok := node.(*js_ast.Expr).Data.(*js_ast.EFunction); ok {
						if signature, ok := reactComponentOrHooks[f.Fn.Body.Loc]; ok {
							signature.hooks = append(signature.hooks, e)
							if !isReactBuiltInHooks(name) {
								signature.customHooks = append(signature.customHooks, target)
							}
							signature.fnBody = &f.Fn.Body
							return
						}
					} else if f, ok := node.(*js_ast.Expr).Data.(*js_ast.EArrow); ok {
						if signature, ok := reactComponentOrHooks[f.Body.Loc]; ok {
							signature.hooks = append(signature.hooks, e)
							if !isReactBuiltInHooks(name) {
								signature.customHooks = append(signature.customHooks, target)
							}
							signature.fnBody = &f.Body
							return
						}
					}
				}
			}
			nodePath = nodePath.ParentPath
		} else {
			return
		}
	}
}

func registerReactHookUse(g *graph.LinkerGraph, ast *js_ast.AST, e *js_ast.Expr, nodePath *js_ast.NodePath) {
	hookReg := regexp.MustCompile("^use[A-Z]")
	switch e.Data.(type) {
	case *js_ast.ECall:
		target := e.Data.(*js_ast.ECall).Target
		switch target.Data.(type) {
		// obj.xxx()
		case *js_ast.EDot:
			name := target.Data.(*js_ast.EDot).Name
			if hookReg.MatchString(name) {
				addReactHooks(g, ast, nodePath, name, *e, target)
			}
		// obj['xxx']()
		case *js_ast.EIndex:
			index := target.Data.(*js_ast.EIndex).Index
			if str, ok := index.Data.(*js_ast.EString); ok {
				name := helpers.UTF16ToString(str.Value)
				if hookReg.MatchString(name) {
					addReactHooks(g, ast, nodePath, name, *e, target)
				}
			}
		// xxx()
		case *js_ast.EIdentifier:
			identifier := target.Data.(*js_ast.EIdentifier)
			symbol := g.Symbols.Get(identifier.Ref)
			if hookReg.MatchString(symbol.OriginalName) {
				addReactHooks(g, ast, nodePath, symbol.OriginalName, *e, target)
			}
		case *js_ast.EImportIdentifier:
			identifier := target.Data.(*js_ast.EImportIdentifier)
			symbol := g.Symbols.Get(identifier.Ref)
			if hookReg.MatchString(symbol.OriginalName) {
				addReactHooks(g, ast, nodePath, symbol.OriginalName, *e, target)
			}
		}

	}
}

func isReactBuiltInHooks(name string) bool {
	switch name {
	case "useState", "useReducer", "useEffect", "useLayoutEffect", "useMemo", "useCallback", "useRef", "useContext", "useImperativeHandle", "useDebugValue":
		return true
	default:
		return false
	}
}

func resetReactRefreshContext() {
	reactComponentOrHooks = make(map[logger.Loc]*ReactSignature)
}

// 第一遍记录组件/hook函数
// 第二遍记录所有存在useHook的函数
// 第三遍生成代码
func registerReactSignature(g *graph.LinkerGraph, ast *js_ast.AST) {
	compReg := regexp.MustCompile("^[A-Z]")
	hookReg := regexp.MustCompile("^use[A-Z]")
	stmts := ast.Parts[1].Stmts[0].Data.(*js_ast.SExpr).Value.Data.(*js_ast.EFunction).Fn.Body.Block.Stmts
	for _, stmt := range stmts {
		switch stmt.Data.(type) {
		case *js_ast.SFunction:
			local := stmt.Data.(*js_ast.SFunction).Fn.Name
			name := g.Symbols.Get(local.Ref).OriginalName
			if compReg.MatchString(name) {
				reactComponentOrHooks[stmt.Data.(*js_ast.SFunction).Fn.Body.Loc] = &ReactSignature{
					binding:     local.Ref,
					hooks:       []js_ast.Expr{},
					customHooks: []js_ast.Expr{},
					isHook:      false,
				}
			} else if hookReg.MatchString(name) {
				reactComponentOrHooks[stmt.Data.(*js_ast.SFunction).Fn.Body.Loc] = &ReactSignature{
					binding:     local.Ref,
					hooks:       []js_ast.Expr{},
					customHooks: []js_ast.Expr{},
					isHook:      true,
				}
			}
		case *js_ast.SLocal:
			decls := stmt.Data.(*js_ast.SLocal).Decls
			for _, decl := range decls {
				value := decl.ValueOrNil
				if id, ok := decl.Binding.Data.(*js_ast.BIdentifier); ok {
					name := g.Symbols.Get(id.Ref).OriginalName
					if arrow, ok := value.Data.(*js_ast.EArrow); ok && (compReg.MatchString(name) || hookReg.MatchString(name)) {
						reactComponentOrHooks[arrow.Body.Loc] = &ReactSignature{
							binding:     id.Ref,
							hooks:       []js_ast.Expr{},
							customHooks: []js_ast.Expr{},
							isHook:      hookReg.MatchString(name),
						}
					} else if fn, ok := value.Data.(*js_ast.EFunction); ok && (compReg.MatchString(name) || hookReg.MatchString(name)) {
						reactComponentOrHooks[fn.Fn.Body.Loc] = &ReactSignature{
							binding:     id.Ref,
							hooks:       []js_ast.Expr{},
							isHook:      hookReg.MatchString(name),
							customHooks: []js_ast.Expr{},
						}
					}
				}
			}
		}
	}

}

func generateReactRefreshCode(log logger.Log, g *graph.LinkerGraph, ast *js_ast.AST) {
	f := ast.Parts[1].Stmts[0].Data.(*js_ast.SExpr).Value.Data.(*js_ast.EFunction)
	signatureCount := 0
	signatureDecls := []js_ast.Stmt{}
	signatureCalls := []js_ast.Stmt{}
	registerCalls := []js_ast.Stmt{}
	for _, signature := range reactComponentOrHooks {
		ref := generateNewSymbol(g, ast, js_ast.SymbolHoisted, fmt.Sprintf("_$s%d", signatureCount))
		signatureCount++
		signatureDecls = append(signatureDecls, js_ast.Stmt{
			Data: &js_ast.SLocal{
				Decls: []js_ast.Decl{
					{
						Binding: js_ast.Binding{
							Data: &js_ast.BIdentifier{
								Ref: ref,
							},
						},
						ValueOrNil: js_ast.Expr{
							Data: &js_ast.ECall{
								Target: js_ast.Expr{
									Data: &js_ast.EDot{
										Target: js_ast.Expr{
											Data: &js_ast.EDot{
												Target: js_ast.Expr{
													Data: &js_ast.EIdentifier{
														Ref: f.Fn.Args[2].Binding.Data.(*js_ast.BIdentifier).Ref,
													},
												},
												Name: "$Refresh$",
											},
										},
										Name: "signature",
									},
								},
							},
						},
					},
				},
			},
		})
		signatureCalls = append(signatureCalls, js_ast.Stmt{
			Data: &js_ast.SExpr{
				Value: js_ast.Expr{
					Data: &js_ast.ECall{
						Target: js_ast.Expr{
							Data: &js_ast.EIdentifier{
								Ref: ref,
							},
						},
						Args: []js_ast.Expr{
							{
								Data: &js_ast.EIdentifier{Ref: signature.binding},
							},
							{
								Data: &js_ast.EString{Value: helpers.StringToUTF16(computeReactSignautreKey(g, ast, *signature))},
							},
							{
								Data: &js_ast.EBoolean{Value: false},
							},
							{
								Data: &js_ast.EFunction{
									Fn: js_ast.Fn{
										Body: js_ast.FnBody{
											Block: js_ast.SBlock{
												Stmts: []js_ast.Stmt{
													{
														Data: &js_ast.SReturn{
															ValueOrNil: js_ast.Expr{
																Data: &js_ast.EArray{Items: signature.customHooks},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		})
		if !signature.isHook {
			registerCalls = append(registerCalls, js_ast.Stmt{
				Data: &js_ast.SExpr{
					Value: js_ast.Expr{
						Data: &js_ast.ECall{
							Target: js_ast.Expr{
								Data: &js_ast.EDot{
									Target: js_ast.Expr{
										Data: &js_ast.EDot{
											Target: js_ast.Expr{
												Data: &js_ast.EIdentifier{
													Ref: f.Fn.Args[2].Binding.Data.(*js_ast.BIdentifier).Ref,
												},
											},
											Name: "$Refresh$",
										},
									},
									Name: "register",
								},
							},
							Args: []js_ast.Expr{
								{
									Data: &js_ast.EIdentifier{Ref: signature.binding},
								},
								{
									Data: &js_ast.EString{Value: helpers.StringToUTF16(g.Symbols.Get(signature.binding).OriginalName)},
								},
							},
						},
					},
				},
			})
		}
		if signature.fnBody != nil {
			signature.fnBody.Block.Stmts = append(signature.fnBody.Block.Stmts[:1], signature.fnBody.Block.Stmts...)
			signature.fnBody.Block.Stmts[0] = js_ast.Stmt{
				Data: &js_ast.SExpr{
					Value: js_ast.Expr{
						Data: &js_ast.ECall{
							Target: js_ast.Expr{
								Data: &js_ast.EIdentifier{
									Ref: ref,
								},
							},
						},
					},
				},
			}
		}
	}
	f.Fn.Body.Block.Stmts = append(f.Fn.Body.Block.Stmts, signatureDecls...)
	f.Fn.Body.Block.Stmts = append(f.Fn.Body.Block.Stmts, signatureCalls...)
	f.Fn.Body.Block.Stmts = append(f.Fn.Body.Block.Stmts, registerCalls...)
	if len(registerCalls) > 0 {
		code := `
	const $ReactRefreshModuleId$ = $$moduleId$$;
	const $ReactRefreshRuntime$ = require("<react_refresh_runtime>");
	const $ReactRefreshCurrentExports$ = $ReactRefreshRuntime$.getModuleExports(
			$ReactRefreshModuleId$
	);

	function $ReactRefreshModuleRuntime$(exports) {
			return $ReactRefreshRuntime$.executeRuntime(
					exports,
					$ReactRefreshModuleId$,
					module.hot,
					false,
					$ReactRefreshModuleId$
			);
	}

	if (typeof Promise !== 'undefined' && $ReactRefreshCurrentExports$ instanceof Promise) {
			$ReactRefreshCurrentExports$.then($ReactRefreshModuleRuntime$);
	} else {
			$ReactRefreshModuleRuntime$($ReactRefreshCurrentExports$);
	}
	`
		refreshAST := js_template.Template(log, ast.ModuleRef.SourceIndex, code, map[string]interface{}{
			"moduleId": &js_ast.EString{
				Value: helpers.StringToUTF16(g.Files[ast.ModuleRef.SourceIndex].InputFile.Source.KeyPath.Text),
			},
		}, js_parser.Options{
			Symbols: ast.Symbols,
		})
		ast.Symbols = refreshAST.Symbols
		g.Symbols.SymbolsForSource[ast.ModuleRef.SourceIndex] = ast.Symbols
		f.Fn.Body.Block.Stmts = append(f.Fn.Body.Block.Stmts, refreshAST.Parts[1].Stmts...)
	}
}

// TODO 协程并行优化
func transformESMToCJS(log logger.Log, g *graph.LinkerGraph, ast *js_ast.AST) {
	resetReactRefreshContext()

	nodeModuleReg := regexp.MustCompile("/node_modules/")
	needReactRefresh := true

	if ast.ModuleRef.SourceIndex < uint32(len(g.Files)) {
		needReactRefresh = !nodeModuleReg.MatchString(g.Files[ast.ModuleRef.SourceIndex].InputFile.Source.KeyPath.Text)
	} else if ast.ModuleRef.SourceIndex > uint32(len(g.Files)) {
		needReactRefresh = false
	}

	if needReactRefresh {
		registerReactSignature(g, ast)
	}

	covertVistor := &js_ast.ASTVisitor{
		VisitStmt: func(s *js_ast.Stmt, nodePath *js_ast.NodePath, iterator *js_ast.StmtIterator) {
			transformImportStmt(g, ast, s)
			transformExportStmt(g, ast, s)
		},
		VisitExpr: func(e *js_ast.Expr, nodePath *js_ast.NodePath) {
			transformImportExpr(g, ast, e)
			if needReactRefresh {
				registerReactHookUse(g, ast, e, nodePath)
			}
		},
	}
	js_ast.TraverseAST(ast.Parts, covertVistor)

	if isESModule {
		markExportESM(g, ast)
	}

	addStmtToFunc(ast, headStmts, false)
	addStmtToFunc(ast, tailStmts, true)

	if needReactRefresh {
		generateReactRefreshCode(log, g, ast)
	}
}

// 这里append的模块，source index都是固定的
func appendModule(log logger.Log, g *graph.LinkerGraph, chunkId string, packModules *[]PackModule, source logger.Source, module *graph.JSRepr) {
	var CJSWrapperAST *js_ast.AST
	var cacheHit bool
	if cacheModule, ok := packModuleCache[source.KeyPath.Text]; ok && cacheModule.Source == source {
		// cache hit
		cacheHit = true
		module = cacheModule.OriginModule
		CJSWrapperAST = cacheModule.AST
		g.Symbols.SymbolsForSource[CJSWrapperAST.ModuleRef.SourceIndex] = CJSWrapperAST.Symbols
	} else {
		// other type file (css or json...)
		if module.AST.HasLazyExport {
			// css
			if module.CSSSourceIndex.IsValid() {
				source = g.Files[module.CSSSourceIndex.GetIndex()].InputFile.Source
				module = transformCSSToJS(log, g, module)
			} else {
				module = transformOtherToJS(module)
			}
		}
		CJSWrapperAST = generateCJS(log, g, module)
		importRecords := *module.ImportRecords()

		// 针对js引入css，改写引入的路径
		for _, record := range importRecords {
			if record.SourceIndex.IsValid() {
				module := g.Files[record.SourceIndex.GetIndex()].InputFile.Repr.(*graph.JSRepr)
				if module.CSSSourceIndex.IsValid() {
					g.Files[record.SourceIndex.GetIndex()].InputFile.Source.KeyPath = g.Files[module.CSSSourceIndex.GetIndex()].InputFile.Source.KeyPath
				}
			}
		}

		headStmts = []js_ast.Stmt{}
		tailStmts = []js_ast.Stmt{}
		importRecordIndexSlice = []*uint32{}
		originImportRecordIndex = []uint32{}
		isESModule = false

		transformESMToCJS(log, g, CJSWrapperAST)

	}

	if !cacheHit {
		cacheModule := PackModule{
			AST:                     CJSWrapperAST,
			Source:                  source,
			OriginImportRecordIndex: originImportRecordIndex,
			ImportRecordIndexSlice:  importRecordIndexSlice,
			OriginModule:            module,
		}
		packModuleCache[source.KeyPath.Text] = cacheModule
		newChunkPackModules[chunkId][source.KeyPath.Text] = cacheModule
		*packModules = append(*packModules, cacheModule)
	} else {
		newChunkPackModules[chunkId][source.KeyPath.Text] = packModuleCache[source.KeyPath.Text]
		*packModules = append(*packModules, packModuleCache[source.KeyPath.Text])
	}

	for _, record := range *module.ImportRecords() {
		index := record.SourceIndex.GetIndex()
		if index == 0 /*runtime code*/ || visitedMap[index] || !record.SourceIndex.IsValid() {
			continue
		}
		visitedMap[index] = true
		nextModule := g.Files[index].InputFile.Repr.(*graph.JSRepr)
		nextSource := g.Files[index].InputFile.Source
		appendModule(log, g, chunkId, packModules, nextSource, nextModule)
	}
}

// TODO 把dev-client移动到graph.files上
func generateEntryChunk(
	log logger.Log,
	g *graph.LinkerGraph,
	chunkId string,
	packCodeAST *js_ast.AST,
	entryPoint graph.EntryPoint,
) *js_ast.AST {
	packModules := make([]PackModule, 0, 64)
	visitedMap = make(map[uint32]bool)

	// add esbuild runtime code
	if repr, ok := g.Files[0].InputFile.Repr.(*graph.JSRepr); ok {
		visitedMap[0] = true
		appendModule(log, g, chunkId, &packModules, g.Files[0].InputFile.Source, repr)
	}

	// add dev client code
	devClientSource := hot.GenerateDevClient()
	if moduleAST, ok := js_parser.Parse(log, devClientSource, js_parser.Options{}); ok {
		g.Symbols.SymbolsForSource[moduleAST.ModuleRef.SourceIndex] = moduleAST.Symbols
		appendModule(log, g, chunkId, &packModules, devClientSource, &graph.JSRepr{
			AST: moduleAST,
		})
	}

	// add style runtime code
	styleRuntimeSource := hot.GenerateStyleRuntime()
	if styleRuntimeAST, ok := js_parser.Parse(log, styleRuntimeSource, js_parser.Options{}); ok {
		g.Symbols.SymbolsForSource[styleRuntimeAST.ModuleRef.SourceIndex] = styleRuntimeAST.Symbols
		appendModule(log, g, chunkId, &packModules, styleRuntimeSource, &graph.JSRepr{
			AST: styleRuntimeAST,
		})
	}

	reactRefreshRuntimeSource := hot.GenerateReactRefershRuntime()
	if reactRefreshRuntimeAST, ok := js_parser.Parse(log, reactRefreshRuntimeSource, js_parser.Options{}); ok {
		g.Symbols.SymbolsForSource[reactRefreshRuntimeAST.ModuleRef.SourceIndex] = reactRefreshRuntimeAST.Symbols
		appendModule(log, g, chunkId, &packModules, reactRefreshRuntimeSource, &graph.JSRepr{
			AST: reactRefreshRuntimeAST,
		})
	}

	// add entry code with modules code
	if repr, ok := g.Files[entryPoint.SourceIndex].InputFile.Repr.(*graph.JSRepr); ok {
		visitedMap[entryPoint.SourceIndex] = true
		appendModule(log, g, chunkId, &packModules, g.Files[entryPoint.SourceIndex].InputFile.Source, repr)

		initialModuleSource, initialModule := generateInitialModule(log, g, entryPoint.SourceIndex)
		appendModule(log, g, chunkId, &packModules, *initialModuleSource, initialModule)

		// pack the modules to chunk
		moreModules := js_ast.EObject{
			Properties: []js_ast.Property{},
		}
		for _, module := range packModules {
			importRecrodStart := uint32(len(packCodeAST.ImportRecords))
			packCodeAST.ImportRecords = append(packCodeAST.ImportRecords, module.AST.ImportRecords...)
			// 更新所有import引用
			for i, index := range module.ImportRecordIndexSlice {
				*index = module.OriginImportRecordIndex[i] + importRecrodStart
			}

			moreModules.Properties = append(moreModules.Properties, js_ast.Property{
				Key: js_ast.Expr{
					Data: &js_ast.EString{
						Value: helpers.StringToUTF16(module.Source.KeyPath.Text),
					},
				},
				ValueOrNil: module.AST.Parts[1].Stmts[0].Data.(*js_ast.SExpr).Value,
			})
		}
		args := &packCodeAST.Parts[1].Stmts[0].Data.(*js_ast.SExpr).Value.Data.(*js_ast.ECall).Args[0]
		args.Data = &moreModules
	}

	return packCodeAST
}

func transformImportStmt(g *graph.LinkerGraph, AST *js_ast.AST, s *js_ast.Stmt) {
	switch s.Data.(type) {
	// import d from 'mod'
	// import { dd } from 'mod'
	case *js_ast.SImport:
		isESModule = true
		importStmt := s.Data.(*js_ast.SImport)
		importRecord := &AST.ImportRecords[importStmt.ImportRecordIndex]
		if importRecord.SourceIndex.IsValid() {
			importRecord.Path = g.Files[importRecord.SourceIndex.GetIndex()].InputFile.Source.KeyPath
		}
		importRecord.Kind = ast.PackRequire
		addImportRecordIndex(&importStmt.ImportRecordIndex)
		if importStmt.DefaultName != nil {
			defaultName := s.Data.(*js_ast.SImport).DefaultName
			name := s.Data.(*js_ast.SImport).NamespaceRef
			if defaultName != nil {
				name = defaultName.Ref
			}
			expr := &js_ast.ERequireString{
				ImportRecordIndex: s.Data.(*js_ast.SImport).ImportRecordIndex,
			}
			addImportRecordIndex(&expr.ImportRecordIndex)
			headStmts = append(headStmts, js_ast.Stmt{
				Data: &js_ast.SLocal{
					Decls: []js_ast.Decl{
						{
							Binding: js_ast.Binding{
								Data: &js_ast.BIdentifier{
									Ref: name,
								},
							},
							ValueOrNil: js_ast.Expr{
								Data: &js_ast.EIf{
									Test: js_ast.Expr{
										Data: &js_ast.EDot{
											Target: js_ast.Expr{
												Data: expr,
											},
											Name: "__esModule",
										},
									},
									Yes: js_ast.Expr{
										Data: &js_ast.EDot{
											Target: js_ast.Expr{
												Data: expr,
											},
											Name: "default",
										},
									},
									No: js_ast.Expr{
										Data: expr,
									},
								},
							},
						},
					},
				},
			})
		}
		if importStmt.Items != nil {
			obj := js_ast.BObject{
				Properties:   []js_ast.PropertyBinding{},
				IsSingleLine: true,
			}
			for _, item := range *s.Data.(*js_ast.SImport).Items {
				bindingName := item.OriginalName
				if len(item.Alias) != 0 {
					bindingName = item.Alias
				}
				obj.Properties = append(obj.Properties, js_ast.PropertyBinding{
					Key: js_ast.Expr{
						Data: &js_ast.EString{
							Value: helpers.StringToUTF16(bindingName),
						},
					},
					Value: js_ast.Binding{
						Data: &js_ast.BIdentifier{
							Ref: item.Name.Ref,
						},
					},
				})
			}
			expr := &js_ast.ERequireString{
				ImportRecordIndex: s.Data.(*js_ast.SImport).ImportRecordIndex,
			}
			addImportRecordIndex(&expr.ImportRecordIndex)
			headStmts = append(headStmts, js_ast.Stmt{
				Data: &js_ast.SLocal{
					Decls: []js_ast.Decl{
						{
							Binding: js_ast.Binding{
								Data: &obj,
							},
							ValueOrNil: js_ast.Expr{
								Data: expr,
							},
						},
					},
				},
			})
		}
		// import "mod"
		// export * as alias from 'mod'
		// SourceIndex 是应该不可能为0的，0是运行时代码
		if importStmt.DefaultName == nil && importStmt.Items == nil && importStmt.NamespaceRef.SourceIndex != 0 {
			symbol := g.Symbols.Get(importStmt.NamespaceRef)
			shouldRemoveNextExportStmt = true
			expr := &js_ast.ERequireString{
				ImportRecordIndex: importStmt.ImportRecordIndex,
			}
			addImportRecordIndex(&expr.ImportRecordIndex)
			if importStmt.StarNameLoc == nil {
				headStmts = append(headStmts, js_ast.Stmt{
					Data: &js_ast.SExpr{
						Value: js_ast.Expr{
							Data: expr,
						},
					},
				})
			} else {
				tailStmts = append(tailStmts, js_ast.Stmt{
					Data: generateExportStmt(AST.ExportsRef, symbol.OriginalName, &js_ast.Expr{
						Data: expr,
					}),
				})
			}
		}
		if importStmt.DefaultName == nil && importStmt.Items == nil && importStmt.NamespaceRef.SourceIndex == 0 {
			expr := &js_ast.ERequireString{
				ImportRecordIndex: importStmt.ImportRecordIndex,
			}
			addImportRecordIndex(&expr.ImportRecordIndex)
			headStmts = append(headStmts, js_ast.Stmt{
				Data: &js_ast.SExpr{
					Value: js_ast.Expr{
						Data: expr,
					},
				},
			})
		}
		// s.Data = &js_ast.SEmpty{}
		s.Data = &js_ast.SComment{Text: "// ESM Transformed"}
	}
}

func transformImportExpr(g *graph.LinkerGraph, AST *js_ast.AST, e *js_ast.Expr) {
	switch e.Data.(type) {
	// require.resolve('mod')
	case *js_ast.ERequireResolveString:
		importRecord := AST.ImportRecords[e.Data.(*js_ast.ERequireResolveString).ImportRecordIndex]
		if importRecord.SourceIndex.IsValid() {
			importRecord.Path = g.Files[importRecord.SourceIndex.GetIndex()].InputFile.Source.KeyPath
		}
		index := &e.Data.(*js_ast.ERequireResolveString).ImportRecordIndex
		addImportRecordIndex(index)
		// require('mod')
	case *js_ast.ERequireString:
		index := &e.Data.(*js_ast.ERequireString).ImportRecordIndex
		importRecord := &AST.ImportRecords[*index]
		if importRecord.SourceIndex.IsValid() {
			importRecord.Path = g.Files[importRecord.SourceIndex.GetIndex()].InputFile.Source.KeyPath
		}
		addImportRecordIndex(index)
		importRecord.Kind = ast.PackRequire
	// import('mod')
	case *js_ast.EDot:
		if _, ok := e.Data.(*js_ast.EDot).Target.Data.(*js_ast.EImportString); ok {
			isESModule = true
			// 这里不需要更新importRecordIndex，交给上面的ERequireResolveString处理
			e.Data.(*js_ast.EDot).Target.Data = &js_ast.ERequireResolveString{
				ImportRecordIndex: e.Data.(*js_ast.EDot).Target.Data.(*js_ast.EImportString).ImportRecordIndex,
			}
		}
	}
}

func generateExportStmt(exportRef js_ast.Ref, name string, value *js_ast.Expr) js_ast.S {
	if value.Data == nil {
		value = &js_ast.Expr{
			Data: &js_ast.EUndefined{},
		}
	}
	return &js_ast.SExpr{
		Value: js_ast.Expr{
			Data: &js_ast.EBinary{
				Op: js_ast.BinOpAssign,
				Left: js_ast.Expr{
					Data: &js_ast.EDot{
						Target: js_ast.Expr{
							Data: &js_ast.EIdentifier{
								Ref: exportRef,
							},
						},
						Name: name,
					},
				},
				Right: *value,
			},
		},
	}
}

func markExportESM(g *graph.LinkerGraph, AST *js_ast.AST) {
	tailStmts = append(tailStmts, js_ast.Stmt{
		Data: generateExportStmt(AST.ExportsRef, "__esModule", &js_ast.Expr{
			Data: &js_ast.EBoolean{Value: true},
		}),
	})
}

func generateAssignToExport(g *graph.LinkerGraph, AST *js_ast.AST, exprs ...js_ast.Expr) js_ast.Expr {
	ref := generateNewSymbol(g, AST, js_ast.SymbolHoisted, "Object")
	return js_ast.Expr{
		Data: &js_ast.ECall{
			Target: js_ast.Expr{
				Data: &js_ast.EDot{
					Target: js_ast.Expr{
						Data: &js_ast.EIdentifier{Ref: ref},
					},
					Name: "assign",
				},
			},
			Args: append([]js_ast.Expr{
				{
					Data: &js_ast.EIdentifier{
						Ref: AST.ExportsRef,
					},
				},
			}, exprs...),
		},
	}
}

func transformExportStmt(g *graph.LinkerGraph, AST *js_ast.AST, s *js_ast.Stmt) {
	switch s.Data.(type) {
	// export default xx
	case *js_ast.SExportDefault:
		isESModule = true
		tailStmts = append(tailStmts, js_ast.Stmt{
			Data: generateExportStmt(AST.ExportsRef, "default", &s.Data.(*js_ast.SExportDefault).Value.Data.(*js_ast.SExpr).Value),
		})
		s.Data = &js_ast.SComment{Text: "// ESM Transformed"}
	// export const x = 123
	case *js_ast.SLocal:
		if s.Data.(*js_ast.SLocal).IsExport {
			isESModule = true
			decl := s.Data.(*js_ast.SLocal).Decls[0]
			ref := decl.Binding.Data.(*js_ast.BIdentifier).Ref
			symbol := g.Symbols.Get(ref)
			s.Data.(*js_ast.SLocal).IsExport = false
			tailStmts = append(tailStmts, js_ast.Stmt{
				Data: generateExportStmt(AST.ExportsRef, symbol.OriginalName, &js_ast.Expr{
					Data: &js_ast.EIdentifier{
						Ref: ref,
					},
				}),
			})
		}
	// export function x() {}
	case *js_ast.SFunction:
		if s.Data.(*js_ast.SFunction).IsExport {
			isESModule = true
			ref := s.Data.(*js_ast.SFunction).Fn.Name.Ref
			symbol := g.Symbols.Get(ref)
			s.Data.(*js_ast.SFunction).IsExport = false
			tailStmts = append(tailStmts, js_ast.Stmt{
				Data: generateExportStmt(AST.ExportsRef, symbol.OriginalName, &js_ast.Expr{
					Data: &js_ast.EIdentifier{
						Ref: ref,
					},
				},
				),
			})
		}
	// export { x: 123 }
	case *js_ast.SExportClause:
		isESModule = true
		if shouldRemoveNextExportStmt {
			s.Data = &js_ast.SComment{Text: "// ESM Transformed"}
			shouldRemoveNextExportStmt = false
			break
		}
		properties := []js_ast.Property{}
		for _, item := range s.Data.(*js_ast.SExportClause).Items {
			name := item.OriginalName
			if item.Alias != "" {
				name = item.Alias
			}
			properties = append(properties, js_ast.Property{
				Key: js_ast.Expr{
					Data: &js_ast.EString{Value: helpers.StringToUTF16(name)},
				},
				ValueOrNil: js_ast.Expr{
					Data: &js_ast.EIdentifier{
						Ref: item.Name.Ref,
					},
				},
			})
		}
		tailStmts = append(tailStmts, js_ast.Stmt{
			Data: &js_ast.SExpr{
				Value: generateAssignToExport(g, AST, js_ast.Expr{Data: &js_ast.EObject{
					Properties: properties,
				}}),
			},
		})
		s.Data = &js_ast.SComment{Text: "// ESM Transformed"}
	case *js_ast.SExportStar:
		isESModule = true
		index := &s.Data.(*js_ast.SExportStar).ImportRecordIndex
		importRecord := &AST.ImportRecords[*index]
		importRecord.Kind = ast.PackRequire
		importRecord.Path = g.Files[importRecord.SourceIndex.GetIndex()].InputFile.Source.KeyPath
		importRecordIndexSlice = append(importRecordIndexSlice, index)
		originImportRecordIndex = append(originImportRecordIndex, *index)

		expr := &js_ast.ERequireString{
			ImportRecordIndex: *index,
		}
		addImportRecordIndex(&expr.ImportRecordIndex)
		// export * as from 'mod'
		if s.Data.(*js_ast.SExportStar).Alias != nil {
			tailStmts = append(tailStmts, js_ast.Stmt{
				Data: generateExportStmt(AST.ExportsRef, s.Data.(*js_ast.SExportStar).Alias.OriginalName, &js_ast.Expr{
					Data: expr,
				},
				),
			})
			// export * form 'mod'
		} else {

			tailStmts = append(tailStmts, js_ast.Stmt{
				Data: &js_ast.SExpr{
					Value: generateAssignToExport(g, AST, js_ast.Expr{Data: expr}),
				},
			})
		}
		s.Data = &js_ast.SComment{Text: "// ESM Transformed"}
	}
}

// index
// ^uint32(0) - 1 dev-client
// ^uint32(0) - 2 style runtime
// ^uint32(0) - 3 react refresh runtime
func generateInitialModule(log logger.Log, g *graph.LinkerGraph, initialModuleIndex uint32) (*logger.Source, *graph.JSRepr) {
	initialModuleId := g.Files[initialModuleIndex].InputFile.Source.KeyPath.Text
	source := logger.Source{
		Index: ^uint32(0) - 3 - initialModuleIndex,
		KeyPath: logger.Path{
			Text: "1",
		},
		Contents: `
			import "<dev_client>"
			import "<react_refresh_runtime>"
			import "` + initialModuleId + `"
		`,
	}
	ast, _ := js_parser.Parse(log, source, js_parser.Options{})
	g.Symbols.SymbolsForSource[ast.ExportsRef.SourceIndex] = ast.Symbols
	return &source, &graph.JSRepr{
		AST: ast,
	}
}

type ModuleState int16

const (
	Add ModuleState = iota
	Replace
	Delete
)

type HotModule struct {
	state  ModuleState
	module PackModule
}

type Manifest struct {
	H string          `json:"h"`
	C map[string]bool `json:"c"`
}

func generateHotCode(log logger.Log, g *graph.LinkerGraph, chunkId string, newPackModuleCache map[string]PackModule, oldPackModuleCache map[string]PackModule) *js_ast.AST {
	hotModules := []HotModule{}

	for path, module := range newPackModuleCache {
		if _, ok := oldPackModuleCache[path]; !ok {
			hotModules = append(hotModules, HotModule{
				state:  Add,
				module: module,
			})
		}
	}

	for path, module := range oldPackModuleCache {
		newModule, ok := newPackModuleCache[path]
		if !ok {
			hotModules = append(hotModules, HotModule{
				state:  Delete,
				module: module,
			})
		} else {
			if module.Source != newPackModuleCache[path].Source {
				hotModules = append(hotModules, HotModule{
					state:  Replace,
					module: newModule,
				})
			}
		}
	}

	if len(hotModules) > 0 {
		hotCodeAST := generateHotUpdateTemplate(log, chunkId)
		callExpr := hotCodeAST.Parts[1].Stmts[0].Data.(*js_ast.SExpr).Value.Data.(*js_ast.ECall)
		moduleObj := callExpr.Args[1].Data.(*js_ast.EObject)
		moduleObj.IsSingleLine = false

		for _, hotModule := range hotModules {
			importRecrodStart := uint32(len(hotCodeAST.ImportRecords))
			hotCodeAST.ImportRecords = append(hotCodeAST.ImportRecords, hotModule.module.AST.ImportRecords...)
			// 更新所有import引用
			for i, index := range hotModule.module.ImportRecordIndexSlice {
				*index = hotModule.module.OriginImportRecordIndex[i] + importRecrodStart
			}

			value := hotModule.module.AST.Parts[1].Stmts[0].Data.(*js_ast.SExpr).Value
			if hotModule.state == Delete {
				value = js_ast.Expr{Data: &js_ast.EBoolean{Value: false}}
			}
			moduleObj.Properties = append(moduleObj.Properties, js_ast.Property{
				Key:        js_ast.Expr{Data: &js_ast.EString{Value: helpers.StringToUTF16(hotModule.module.Source.KeyPath.Text)}},
				ValueOrNil: value,
			})
		}
		g.Symbols.SymbolsForSource[hotCodeAST.ExportsRef.SourceIndex] = hotCodeAST.Symbols
		return hotCodeAST
	}
	return nil
}

func printJS(ast *js_ast.AST, g *graph.LinkerGraph) js_printer.PrintResult {
	r := renamer.NewNoOpRenamer(g.Symbols)
	return js_printer.Print(*ast, g.Symbols, r, js_printer.Options{
		OutputFormat: config.FormatCommonJS,
		RequireOrImportMetaForSource: func(u uint32) js_printer.RequireOrImportMeta {
			return js_printer.RequireOrImportMeta{}
		},
	})
}

func PackLink(
	options *config.Options,
	timer *helpers.Timer,
	log logger.Log,
	fs fs.FS,
	res *resolver.Resolver,
	inputFiles []graph.InputFile,
	entryPoints []graph.EntryPoint,
	uniqueKeyPrefix string,
	reachableFiles []uint32,
	dataForSourceMaps func() []bundler.DataForSourceMap,
) (results []graph.OutputFile) {

	// clone的时候会把语法树上的symbols设为nil
	g := graph.CloneLinkerGraph(
		inputFiles,
		reachableFiles,
		entryPoints,
		options.CodeSplitting,
	)

	newHash := randHash(16)

	manifest := Manifest{
		H: newHash,
		C: map[string]bool{},
	}

	// 并行生成chunk
	for _, entryPoint := range entryPoints {
		chunkId := inputFiles[entryPoint.SourceIndex].Source.IdentifierName

		newChunkPackModules[chunkId] = make(map[string]PackModule)

		// initial module id is default "1"
		runtimeSource := runtime.GeneratePackSource(js_ast.MakeDynamicIndex(), chunkId, "1", manifest.H)
		packCodeAST, _ := js_parser.Parse(log, runtimeSource, js_parser.Options{})
		g.Symbols.SymbolsForSource[runtimeSource.Index] = packCodeAST.Symbols
		ast := generateEntryChunk(log, &g, chunkId, &packCodeAST, entryPoint)
		result := printJS(ast, &g)

		if currentHash != "" {
			// 生成hot code
			hotCode := generateHotCode(log, &g, chunkId, newChunkPackModules[chunkId], chunkPackModules[chunkId])
			if hotCode != nil {
				manifest.C[chunkId] = true
				results = append(results, graph.OutputFile{
					AbsPath:  fs.Join(options.AbsOutputDir, chunkId+"."+currentHash+".hot-update.js"),
					Contents: printJS(hotCode, &g).JS,
				})
			} else {
				manifest.C[chunkId] = false
			}
		}

		chunkPackModules = newChunkPackModules
		newChunkPackModules = map[string]map[string]PackModule{}

		ext := fs.Ext(entryPoint.OutputPath)
		if ext != "" {
			ext = ""
		} else {
			ext = ".js"
		}
		results = append(results, graph.OutputFile{
			AbsPath:  fs.Join(options.AbsOutputDir, entryPoint.OutputPath+ext),
			Contents: result.JS,
		})
	}

	if currentHash != "" {
		// 添加maifest.json
		manifestContent, _ := json.Marshal(manifest)
		results = append(results, graph.OutputFile{
			AbsPath:  fs.Join(options.AbsOutputDir, currentHash+".hot-update.json"),
			Contents: manifestContent,
		})
	}

	results = append(results, graph.OutputFile{
		AbsPath:  fs.Join(options.AbsOutputDir, "versions"),
		Contents: []byte(manifest.H),
	})
	// 更新hash
	currentHash = manifest.H
	println("current hash:", currentHash)
	return results
}
