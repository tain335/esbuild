package streamlinker

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/evanw/esbuild/internal/ast"
	"github.com/evanw/esbuild/internal/config"
	"github.com/evanw/esbuild/internal/helpers"
	"github.com/evanw/esbuild/internal/hmr"
	"github.com/evanw/esbuild/internal/js_ast"
	"github.com/evanw/esbuild/internal/js_parser"
	"github.com/evanw/esbuild/internal/js_template"
	"github.com/evanw/esbuild/internal/logger"
	"github.com/evanw/esbuild/internal/resolver"
	"github.com/evanw/esbuild/internal/sourcemap"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("0123456789abcdefghijklmnopqrstuvwxyz")

func RandHash(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

type ModuleReactSignature struct {
	binding     js_ast.Ref
	hooks       []js_ast.Expr
	customHooks []js_ast.Expr
	fnBody      *js_ast.FnBody
	isHook      bool
}

type ModuleTransformerContext struct {
	ast                         *js_ast.AST
	source                      logger.Source
	log                         logger.Log
	headStmts                   []js_ast.Stmt
	tailStmts                   []js_ast.Stmt
	resolveResult               []*resolver.ResolveResult
	symbols                     *js_ast.SymbolMap
	renameIdentifierExportCache map[js_ast.Ref]js_ast.Expr
	reactComponentOrHooks       map[logger.Loc]*ModuleReactSignature
	runtimeSourceCache          map[string]uint32
	moduleRef                   js_ast.Ref
	exportsRef                  js_ast.Ref
	requireRef                  js_ast.Ref
	options                     *config.Options
}

type ModuleTransformResult struct {
	JS        string
	SourceMap sourcemap.Chunk
}

func NewModuleTransformerContext(
	log logger.Log,
	source logger.Source,
	symbols *js_ast.SymbolMap,
	resolveResult []*resolver.ResolveResult,
	runtimeSourceCache map[string]uint32,
	ast *js_ast.AST,
	options *config.Options,
) *ModuleTransformerContext {
	requireRef := generateNewSymbol(symbols, ast, js_ast.SymbolHoisted, "require")
	return &ModuleTransformerContext{
		source:                      source,
		ast:                         ast,
		log:                         log,
		symbols:                     symbols,
		runtimeSourceCache:          runtimeSourceCache,
		headStmts:                   make([]js_ast.Stmt, 0),
		tailStmts:                   make([]js_ast.Stmt, 0),
		resolveResult:               resolveResult,
		renameIdentifierExportCache: make(map[js_ast.Ref]js_ast.Expr),
		reactComponentOrHooks:       make(map[logger.Loc]*ModuleReactSignature),
		moduleRef:                   ast.ModuleRef,
		exportsRef:                  ast.ExportsRef,
		requireRef:                  requireRef,
		options:                     &config.Options{},
	}
}

func (c *ModuleTransformerContext) registerReactSignature() {
	compReg := regexp.MustCompile("^[A-Z]")
	hookReg := regexp.MustCompile("^use[A-Z]")
	stmts := c.ast.Parts[0].Stmts
	var register func(stmt js_ast.Stmt)
	register = func(stmt js_ast.Stmt) {
		switch s := stmt.Data.(type) {
		case *js_ast.SFunction:
			local := s.Fn.Name
			name := c.symbols.Get(local.Ref).OriginalName
			if compReg.MatchString(name) {
				c.reactComponentOrHooks[s.Fn.Body.Loc] = &ModuleReactSignature{
					binding:     local.Ref,
					hooks:       []js_ast.Expr{},
					customHooks: []js_ast.Expr{},
					isHook:      false,
				}
			} else if hookReg.MatchString(name) {
				c.reactComponentOrHooks[s.Fn.Body.Loc] = &ModuleReactSignature{
					binding:     local.Ref,
					hooks:       []js_ast.Expr{},
					customHooks: []js_ast.Expr{},
					isHook:      true,
				}
			}
		case *js_ast.SLocal:
			decls := s.Decls
			for _, decl := range decls {
				value := decl.ValueOrNil
				if id, ok := decl.Binding.Data.(*js_ast.BIdentifier); ok {
					name := c.symbols.Get(id.Ref).OriginalName
					if arrow, ok := value.Data.(*js_ast.EArrow); ok && (compReg.MatchString(name) || hookReg.MatchString(name)) {
						c.reactComponentOrHooks[arrow.Body.Loc] = &ModuleReactSignature{
							binding:     id.Ref,
							hooks:       []js_ast.Expr{},
							customHooks: []js_ast.Expr{},
							isHook:      hookReg.MatchString(name),
						}
					} else if fn, ok := value.Data.(*js_ast.EFunction); ok && (compReg.MatchString(name) || hookReg.MatchString(name)) {
						c.reactComponentOrHooks[fn.Fn.Body.Loc] = &ModuleReactSignature{
							binding:     id.Ref,
							hooks:       []js_ast.Expr{},
							isHook:      hookReg.MatchString(name),
							customHooks: []js_ast.Expr{},
						}
					}
				}
			}
		case *js_ast.SExportDefault:
			register(s.Value)
		}
	}
	for _, stmt := range stmts {
		register(stmt)
	}
}

func (c *ModuleTransformerContext) generateExportStmt(exportRef js_ast.Ref, name string, value *js_ast.Expr) js_ast.S {
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

func (c *ModuleTransformerContext) markExportESM() {
	c.tailStmts = append(c.tailStmts, js_ast.Stmt{
		Data: c.generateExportStmt(c.ast.ExportsRef, "__esModule", &js_ast.Expr{
			Data: &js_ast.EBoolean{Value: true},
		}),
	})
}

func (c *ModuleTransformerContext) generateNewSymbol(kind js_ast.SymbolKind, originalName string) js_ast.Ref {
	return generateNewSymbol(c.symbols, c.ast, kind, originalName)
}

func (c *ModuleTransformerContext) transformImportStmt(s *js_ast.Stmt) {
	switch s.Data.(type) {
	// import d from 'mod'
	// import { d } from 'mod'
	// import * as d from 'mod'
	case *js_ast.SImport:
		importStmt := s.Data.(*js_ast.SImport)
		importRecord := &c.ast.ImportRecords[importStmt.ImportRecordIndex]
		isESM := importRecord.Kind == ast.ImportStmt || importRecord.Kind == ast.ImportDynamic
		if importStmt.DefaultName != nil {
			defaultNameRef := importStmt.DefaultName.Ref

			expr := &js_ast.ERequireString{
				ImportRecordIndex: s.Data.(*js_ast.SImport).ImportRecordIndex,
			}

			value := js_ast.Expr{Data: expr}

			if isESM {
				value = js_ast.Expr{
					Data: &js_ast.ECall{
						Target: js_ast.Expr{
							Data: &js_ast.EDot{
								Target: js_ast.Expr{
									Data: &js_ast.EIdentifier{
										Ref: c.requireRef,
									},
								},
								Name: "n",
							},
						},
						Args: []js_ast.Expr{
							{
								Data: expr,
							},
						},
					},
				}
			}

			c.headStmts = append(c.headStmts, js_ast.Stmt{
				Data: &js_ast.SLocal{
					Decls: []js_ast.Decl{
						{
							Binding: js_ast.Binding{
								Data: &js_ast.BIdentifier{
									Ref: defaultNameRef,
								},
							},
							ValueOrNil: value,
						},
					},
				},
			})
			newDefaultRef := c.generateNewSymbol(js_ast.SymbolHoisted, c.ast.Symbols[importStmt.DefaultName.Ref.InnerIndex].OriginalName)
			c.renameIdentifierExportCache[defaultNameRef] = js_ast.Expr{
				Data: &js_ast.ECall{
					Target: js_ast.Expr{
						Data: &js_ast.EIdentifier{
							Ref: newDefaultRef,
						},
					},
				},
			}
		}
		if importStmt.Items != nil {
			expr := &js_ast.ERequireString{
				ImportRecordIndex: importStmt.ImportRecordIndex,
			}
			if isESM {
				var importModName string = fmt.Sprintf("_ESPACK_MODULE_%d", importStmt.ImportRecordIndex)
				importModRef := c.generateNewSymbol(js_ast.SymbolHoisted, importModName)
				// 解决循环引用模块问题
				for _, item := range *s.Data.(*js_ast.SImport).Items {
					c.renameIdentifierExportCache[item.Name.Ref] = js_ast.Expr{
						Data: &js_ast.EDot{
							Target: js_ast.Expr{
								Data: &js_ast.EIdentifier{
									Ref: importModRef,
								},
							},
							Name: item.Alias,
						},
					}
				}

				c.headStmts = append(c.headStmts, js_ast.Stmt{
					Data: &js_ast.SLocal{
						Decls: []js_ast.Decl{
							{
								Binding: js_ast.Binding{
									Data: &js_ast.BIdentifier{
										Ref: importModRef,
									},
								},
								ValueOrNil: js_ast.Expr{
									Data: expr,
								},
							},
						},
					},
				})
			} else {
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

				c.headStmts = append(c.headStmts, js_ast.Stmt{
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
		}

		// import "mod"
		// export * as alias from 'mod'
		// SourceIndex 是应该不可能为0的，0是运行时代码
		if importStmt.DefaultName == nil && importStmt.Items == nil && importStmt.NamespaceRef.SourceIndex != 0 {
			expr := &js_ast.ERequireString{
				ImportRecordIndex: importStmt.ImportRecordIndex,
			}

			if importStmt.StarNameLoc == nil {
				c.headStmts = append(c.headStmts, js_ast.Stmt{
					Data: &js_ast.SExpr{
						Value: js_ast.Expr{
							Data: expr,
						},
					},
				})
			} else {
				c.headStmts = append(c.headStmts, js_ast.Stmt{
					Data: &js_ast.SLocal{
						Decls: []js_ast.Decl{
							{
								Binding: js_ast.Binding{
									Data: &js_ast.BIdentifier{
										Ref: importStmt.NamespaceRef,
									},
								},
								ValueOrNil: js_ast.Expr{
									Data: expr,
								},
							},
						},
					},
				})

				// c.tailStmts = append(c.tailStmts, js_ast.Stmt{
				// 	Data: c.generateExportStmt(c.ast.ExportsRef, symbol.OriginalName, &js_ast.Expr{
				// 		Data: expr,
				// 	}),
				// })
			}
		}
		if importStmt.DefaultName == nil && importStmt.Items == nil && importStmt.NamespaceRef.SourceIndex == 0 {
			expr := &js_ast.ERequireString{
				ImportRecordIndex: importStmt.ImportRecordIndex,
			}
			// c.addImportRecordIndex(&expr.ImportRecordIndex)
			c.headStmts = append(c.headStmts, js_ast.Stmt{
				Data: &js_ast.SExpr{
					Value: js_ast.Expr{
						Data: expr,
					},
				},
			})
		}

		s.Data = &js_ast.SComment{Text: "// ESM Transformed"}
	}
}

func (c *ModuleTransformerContext) transformRenameExpr(e *js_ast.Expr) {
	var tryToRename = func(ref js_ast.Ref, e *js_ast.Expr) {
		if defaultExpr, ok := c.renameIdentifierExportCache[ref]; ok {
			e.Data = defaultExpr.Data
		}
	}
	switch expr := e.Data.(type) {
	case *js_ast.EImportIdentifier:
		tryToRename(expr.Ref, e)
	case *js_ast.EIdentifier:
		tryToRename(expr.Ref, e)
	}
}

func (c *ModuleTransformerContext) transformImportExpr(e *js_ast.Expr) {
	switch e.Data.(type) {
	case *js_ast.EDot:
		if _, ok := e.Data.(*js_ast.EDot).Target.Data.(*js_ast.EImportString); ok {
			c.ast.ExportsKind = js_ast.ExportsESMWithDynamicFallback
			e.Data.(*js_ast.EDot).Target.Data = &js_ast.ERequireResolveString{
				ImportRecordIndex: e.Data.(*js_ast.EDot).Target.Data.(*js_ast.EImportString).ImportRecordIndex,
			}
		}
	}
}

func (c *ModuleTransformerContext) generateAssignToExport(exprs ...js_ast.Expr) js_ast.Expr {
	ref := c.generateNewSymbol(js_ast.SymbolHoisted, "Object")
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
						Ref: c.ast.ExportsRef,
					},
				},
			}, exprs...),
		},
	}
}

func (c *ModuleTransformerContext) isReactBuiltInHooks(name string) bool {
	switch name {
	case "useState", "useReducer", "useEffect", "useLayoutEffect", "useMemo", "useCallback", "useRef", "useContext", "useImperativeHandle", "useDebugValue":
		return true
	default:
		return false
	}
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
func (c *ModuleTransformerContext) addReactHooks(nodePath *js_ast.NodePath, name string, e js_ast.Expr, target js_ast.Expr) {
	for {
		if nodePath.Node != nil {
			node := nodePath.Node
			switch node.(type) {
			case *js_ast.Stmt:
				if f, ok := node.(*js_ast.Stmt).Data.(*js_ast.SFunction); ok {
					if signature, ok := c.reactComponentOrHooks[f.Fn.Body.Loc]; ok {
						signature.hooks = append(signature.hooks, e)
						if !c.isReactBuiltInHooks(name) {
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
						if signature, ok := c.reactComponentOrHooks[f.Fn.Body.Loc]; ok {
							signature.hooks = append(signature.hooks, e)
							if !c.isReactBuiltInHooks(name) {
								signature.customHooks = append(signature.customHooks, target)
							}
							signature.fnBody = &f.Fn.Body
							return
						}
					} else if f, ok := node.(*js_ast.Expr).Data.(*js_ast.EArrow); ok {
						if signature, ok := c.reactComponentOrHooks[f.Body.Loc]; ok {
							signature.hooks = append(signature.hooks, e)
							if !c.isReactBuiltInHooks(name) {
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

func (c *ModuleTransformerContext) registerReactHookUse(e *js_ast.Expr, nodePath *js_ast.NodePath) {
	hookReg := regexp.MustCompile("^use[A-Z]")
	switch expr := e.Data.(type) {
	case *js_ast.ECall:
		target := expr.Target
		switch target.Data.(type) {
		// obj.xxx()
		case *js_ast.EDot:
			name := target.Data.(*js_ast.EDot).Name
			if hookReg.MatchString(name) {
				c.addReactHooks(nodePath, name, *e, target)
			}
		// obj['xxx']()
		case *js_ast.EIndex:
			index := target.Data.(*js_ast.EIndex).Index
			if str, ok := index.Data.(*js_ast.EString); ok {
				name := helpers.UTF16ToString(str.Value)
				if hookReg.MatchString(name) {
					c.addReactHooks(nodePath, name, *e, target)
				}
			}
		// xxx()
		case *js_ast.EIdentifier:
			identifier := target.Data.(*js_ast.EIdentifier)
			symbol := c.symbols.Get(identifier.Ref)
			if hookReg.MatchString(symbol.OriginalName) {
				c.addReactHooks(nodePath, symbol.OriginalName, *e, target)
			}
		case *js_ast.EImportIdentifier:
			identifier := target.Data.(*js_ast.EImportIdentifier)
			symbol := c.symbols.Get(identifier.Ref)
			if hookReg.MatchString(symbol.OriginalName) {
				c.addReactHooks(nodePath, symbol.OriginalName, *e, target)
			}
		}

	}
}

func (c *ModuleTransformerContext) transformExportStmt(s *js_ast.Stmt) {
	switch s.Data.(type) {
	// export { xxx } from "xxx"
	case *js_ast.SExportFrom:
		// isESModule = true
		exportFrom := s.Data.(*js_ast.SExportFrom)
		s.Data = &js_ast.SComment{Text: "// ESM Transformed"}
		for _, item := range exportFrom.Items {
			require := &js_ast.ERequireString{
				ImportRecordIndex: exportFrom.ImportRecordIndex,
			}
			// c.rewriteImportRecord(&require.ImportRecordIndex)
			c.tailStmts = append(c.tailStmts, js_ast.Stmt{
				Data: c.generateExportStmt(c.ast.ExportsRef, item.Alias, &js_ast.Expr{
					Loc: s.Loc,
					Data: &js_ast.EDot{
						Target: js_ast.Expr{
							Data: require,
							Loc:  s.Loc,
						},
						Name: item.OriginalName,
					},
				}),
				Loc: s.Loc,
			})

		}

	// export default xx
	case *js_ast.SExportDefault:
		// isESModule = true
		exportDefault := &s.Data.(*js_ast.SExportDefault).Value
		switch exportDefault.Data.(type) {
		case *js_ast.SExpr:
			c.tailStmts = append(c.tailStmts, js_ast.Stmt{
				Data: c.generateExportStmt(c.ast.ExportsRef, "default", &exportDefault.Data.(*js_ast.SExpr).Value),
				Loc:  s.Loc,
			})
			s.Data = &js_ast.SComment{Text: "// ESM Transformed"}
		case *js_ast.SFunction:
			if exportDefault.Data.(*js_ast.SFunction).Fn.Name != nil {
				s.Data = exportDefault.Data
				c.tailStmts = append(c.tailStmts, js_ast.Stmt{
					Data: c.generateExportStmt(c.ast.ExportsRef, "default", &js_ast.Expr{
						Loc: s.Loc,
						Data: &js_ast.EIdentifier{
							Ref: exportDefault.Data.(*js_ast.SFunction).Fn.Name.Ref,
						},
					}),
				})
			} else {
				c.tailStmts = append(c.tailStmts, js_ast.Stmt{
					Data: c.generateExportStmt(c.ast.ExportsRef, "default", &js_ast.Expr{
						Loc:  s.Loc,
						Data: &js_ast.EFunction{Fn: exportDefault.Data.(*js_ast.SFunction).Fn},
					}),
				})
				s.Data = &js_ast.SComment{Text: "// ESM Transformed"}
			}
		}

	// export const x = 123
	case *js_ast.SLocal:
		if s.Data.(*js_ast.SLocal).IsExport {
			s.Data.(*js_ast.SLocal).IsExport = false
			for _, decl := range s.Data.(*js_ast.SLocal).Decls {
				ref := decl.Binding.Data.(*js_ast.BIdentifier).Ref
				symbol := c.symbols.Get(ref)

				c.tailStmts = append(c.tailStmts, js_ast.Stmt{
					Data: c.generateExportStmt(c.ast.ExportsRef, symbol.OriginalName, &js_ast.Expr{
						Data: &js_ast.EIdentifier{
							Ref: ref,
						},
					}),
					Loc: s.Loc,
				})
			}
		}
	// export function x() {}
	case *js_ast.SFunction:
		if s.Data.(*js_ast.SFunction).IsExport {
			ref := s.Data.(*js_ast.SFunction).Fn.Name.Ref
			symbol := c.symbols.Get(ref)
			s.Data.(*js_ast.SFunction).IsExport = false
			c.tailStmts = append(c.tailStmts, js_ast.Stmt{
				Data: c.generateExportStmt(c.ast.ExportsRef, symbol.OriginalName, &js_ast.Expr{
					Data: &js_ast.EIdentifier{
						Ref: ref,
					},
					Loc: s.Loc,
				},
				),
			})
		}
	// export { x: 123 }
	case *js_ast.SExportClause:
		properties := []js_ast.Property{}
		for _, item := range s.Data.(*js_ast.SExportClause).Items {
			name := item.OriginalName
			if item.Alias != "" {
				name = item.Alias
			}
			expr := js_ast.Expr{
				Data: &js_ast.EIdentifier{
					Ref: item.Name.Ref,
				},
			}
			if e, ok := c.renameIdentifierExportCache[item.Name.Ref]; ok {
				expr = e
			}
			properties = append(properties, js_ast.Property{
				Key: js_ast.Expr{
					Data: &js_ast.EString{Value: helpers.StringToUTF16(name)},
				},
				ValueOrNil: expr,
			})
		}
		c.tailStmts = append(c.tailStmts, js_ast.Stmt{
			Data: &js_ast.SExpr{
				Value: c.generateAssignToExport(js_ast.Expr{Data: &js_ast.EObject{
					Properties: properties,
				}}),
			},
			Loc: s.Loc,
		})
		s.Data = &js_ast.SComment{Text: "// ESM Transformed"}
	case *js_ast.SExportStar:
		c.ast.ExportsKind = js_ast.ExportsESMWithDynamicFallback
		index := &s.Data.(*js_ast.SExportStar).ImportRecordIndex

		expr := &js_ast.ERequireString{
			ImportRecordIndex: *index,
		}
		// export * as xxx from 'mod'
		if s.Data.(*js_ast.SExportStar).Alias != nil {
			c.tailStmts = append(c.tailStmts, js_ast.Stmt{
				Data: c.generateExportStmt(c.ast.ExportsRef, s.Data.(*js_ast.SExportStar).Alias.OriginalName, &js_ast.Expr{
					Data: expr,
				},
				),
			})
			// export * form 'mod'
		} else {
			c.tailStmts = append(c.tailStmts, js_ast.Stmt{
				Data: &js_ast.SExpr{
					Value: c.generateAssignToExport(js_ast.Expr{Data: expr}),
				},
			})
		}
		s.Data = &js_ast.SComment{Text: "// ESM Transformed"}
	}
}

func (c *ModuleTransformerContext) addStmtToFunc(stmts []js_ast.Stmt, tail bool) {
	partStmts := c.ast.Parts[0].Stmts
	if tail {
		c.ast.Parts[0].Stmts = append(partStmts, stmts...)
	} else {
		c.ast.Parts[0].Stmts = append(stmts, partStmts...)
	}
}

func (c *ModuleTransformerContext) computeReactSignautreKey(signature ModuleReactSignature) string {
	key := ""
	for _, hook := range signature.hooks {
		newAst := *c.ast
		newAst.Parts = []js_ast.Part{
			{
				Stmts: []js_ast.Stmt{{
					Data: &js_ast.SExpr{
						Value: hook,
					},
				}},
			},
		}
		key += string(printJS(c.source, &newAst, c.symbols, c.options).JS)
	}
	hash := md5.Sum([]byte(key))
	md5str := fmt.Sprintf("%x", hash)
	return md5str
}

func (c *ModuleTransformerContext) generateReactRefreshCode() {
	signatureCount := 0
	signatureDecls := []js_ast.Stmt{}
	signatureCalls := []js_ast.Stmt{}
	registerCalls := []js_ast.Stmt{}
	for _, signature := range c.reactComponentOrHooks {
		ref := c.generateNewSymbol(js_ast.SymbolHoisted, fmt.Sprintf("_$s%d", signatureCount))
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
														Ref: c.requireRef,
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
								Data: &js_ast.EString{Value: helpers.StringToUTF16(c.computeReactSignautreKey(*signature))},
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
													Ref: c.requireRef,
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
									Data: &js_ast.EString{Value: helpers.StringToUTF16(c.symbols.Get(signature.binding).OriginalName)},
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
	c.ast.Parts[0].Stmts = append(c.ast.Parts[0].Stmts, signatureDecls...)
	c.ast.Parts[0].Stmts = append(c.ast.Parts[0].Stmts, signatureCalls...)
	c.ast.Parts[0].Stmts = append(c.ast.Parts[0].Stmts, registerCalls...)

	if len(registerCalls) > 0 {
		code := `
	// const $ReactRefreshRuntime$ = require("<react_refresh_runtime>");
	const $ReactRefreshModuleId$ = $$moduleId$$;
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
		refreshAST := js_template.Template(c.log, js_ast.MakeDynamicIndex(), code, map[string]interface{}{
			"moduleId": &js_ast.EString{
				Value: helpers.StringToUTF16(c.source.KeyPath.Text),
			},
		}, js_parser.Options{})

		c.symbols.SymbolsForSource[refreshAST.ModuleRef.SourceIndex] = refreshAST.Symbols

		refreshAST = combineParts(refreshAST)

		c.tailStmts = append(c.tailStmts, refreshAST.Parts[0].Stmts...)

		ref := c.generateNewSymbol(js_ast.SymbolHoisted, "$ReactRefreshRuntime$")
		c.ast.ImportRecords = append(c.ast.ImportRecords, ast.ImportRecord{
			Kind:        ast.PackRequire,
			SourceIndex: ast.MakeIndex32(c.runtimeSourceCache[hmr.ReactRefreshRuntimePath.Text]),
			Path:        hmr.ReactRefreshRuntimePath,
		})
		c.headStmts = append(c.headStmts, js_ast.Stmt{
			Data: &js_ast.SLocal{
				Decls: []js_ast.Decl{
					{
						Binding: js_ast.Binding{
							Data: &js_ast.BIdentifier{
								Ref: ref,
							},
						},
						ValueOrNil: js_ast.Expr{
							Data: &js_ast.ERequireString{
								ImportRecordIndex: uint32(len(c.ast.ImportRecords)) - 1,
							},
						},
					},
				},
			},
		})
	}
}

func (c *ModuleTransformerContext) wrapperCJS() {
	directive := c.ast.Directive
	c.ast.Directive = ""

	mainPart := &c.ast.Parts[0]

	var stmts []js_ast.Stmt
	if directive != "" {
		stmts = append([]js_ast.Stmt{
			{
				Data: &js_ast.SDirective{
					Value: helpers.StringToUTF16(directive),
				},
			},
		}, mainPart.Stmts...)
	} else {
		stmts = mainPart.Stmts
	}
	c.ast.Parts[0].Stmts = []js_ast.Stmt{
		{
			Data: &js_ast.SExpr{
				Value: js_ast.Expr{
					Data: &js_ast.EFunction{
						Fn: js_ast.Fn{
							Args: []js_ast.Arg{
								{
									Binding: js_ast.Binding{
										Data: &js_ast.BIdentifier{
											Ref: c.moduleRef,
										},
									},
								},
								{
									Binding: js_ast.Binding{
										Data: &js_ast.BIdentifier{
											Ref: c.exportsRef,
										},
									},
								},
								{
									Binding: js_ast.Binding{
										Data: &js_ast.BIdentifier{
											Ref: c.requireRef,
										},
									},
								},
							},
							Body: js_ast.FnBody{
								Block: js_ast.SBlock{
									Stmts: stmts,
								},
							},
						},
					},
				},
			},
		},
	}
}

func (c *ModuleTransformerContext) output() ModuleTransformResult {
	c.wrapperCJS()
	pr := printJS(c.source, c.ast, c.symbols, c.options)
	return ModuleTransformResult{
		JS:        strings.TrimRight(string(pr.JS), ";\n"),
		SourceMap: pr.SourceMapChunk,
	}
}

func (c *ModuleTransformerContext) transformOtherToCJS() ModuleTransformResult {
	c.ast = combineParts(c.ast)
	covertVistor := &js_ast.ASTVisitor{
		EnterStmt: func(s *js_ast.Stmt, path *js_ast.NodePath, iterator *js_ast.StmtIterator) {
			if lazy, ok := s.Data.(*js_ast.SLazyExport); ok {
				*s = js_ast.AssignStmt(
					js_ast.Expr{Loc: lazy.Value.Loc, Data: &js_ast.EDot{
						Target:  js_ast.Expr{Loc: lazy.Value.Loc, Data: &js_ast.EIdentifier{Ref: c.ast.ModuleRef}},
						Name:    "exports",
						NameLoc: lazy.Value.Loc,
					}},
					lazy.Value,
				)
			}
		},
	}

	js_ast.TraverseAST(c.ast.Parts, covertVistor)

	return c.output()
}

func (c *ModuleTransformerContext) transformESMToCJS() ModuleTransformResult {
	c.ast = combineParts(c.ast)
	needReactRefresh := false

	if !helpers.IsInternalModule(c.source.KeyPath.Text) && !helpers.IsInsideNodeModules(c.source.KeyPath.Text) {
		needReactRefresh = true
	}

	if needReactRefresh {
		c.registerReactSignature()
	}

	covertVistor := &js_ast.ASTVisitor{
		EnterStmt: func(s *js_ast.Stmt, path *js_ast.NodePath, iterator *js_ast.StmtIterator) {
			c.transformImportStmt(s)
		},
		ExitStmt: func(s *js_ast.Stmt, nodePath *js_ast.NodePath, iterator *js_ast.StmtIterator) {
			c.transformExportStmt(s)
		},
		EnterExpr: func(e *js_ast.Expr, nodePath *js_ast.NodePath) {
			c.transformImportExpr(e)
			c.transformRenameExpr(e)
			if needReactRefresh {
				c.registerReactHookUse(e, nodePath)
			}
		},
	}

	js_ast.TraverseAST(c.ast.Parts, covertVistor)

	if c.ast.ExportsKind == js_ast.ExportsESM || c.ast.ExportsKind == js_ast.ExportsESMWithDynamicFallback {
		c.markExportESM()
	}

	if needReactRefresh {
		c.generateReactRefreshCode()
	}

	c.addStmtToFunc(c.headStmts, false)
	c.addStmtToFunc(c.tailStmts, true)

	for index := range c.ast.ImportRecords {
		imporRecord := &c.ast.ImportRecords[index]
		if imporRecord.Kind == ast.PackRequire {
			continue
		}
		if imporRecord.SourceIndex.GetIndex() == 0 {
			imporRecord.Kind = ast.PackRequire
			continue
		}
		result := c.resolveResult[index]
		if result == nil {
			continue
		}
		imporRecord.Kind = ast.PackRequire
		imporRecord.Path.Text = result.PathPair.Primary.Text
	}

	return c.output()
}
