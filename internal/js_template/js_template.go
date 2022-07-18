package js_template

import (
	"strings"

	"github.com/evanw/esbuild/internal/js_ast"
	"github.com/evanw/esbuild/internal/js_parser"
	"github.com/evanw/esbuild/internal/logger"
)

// eg:
// js_template.Template(
// 	log,
// 	"function aa() { $$str$$ }",
// 	map[string]interface{}{
// 		"str": &js_ast.SExpr{
// 			Value: js_ast.Expr{
// 				Data: &js_ast.ENumber{
// 					Value: 60,
// 				},
// 			},
// 		},
// 	}, make([]js_ast.Symbol, 0), js_parser.Options{})
func Template(log logger.Log, sourceIndex uint32, template string, data map[string]interface{}, options js_parser.Options) *js_ast.AST {
	ast, _ := js_parser.Parse(log, logger.Source{
		Index:          sourceIndex,
		KeyPath:        logger.Path{Text: "<template>"},
		PrettyPath:     "<template>",
		IdentifierName: "template",
		Contents:       template,
	}, options)
	js_ast.TraverseAST(ast.Parts, &js_ast.ASTVisitor{
		VisitStmt: func(s *js_ast.Stmt, nodePath *js_ast.NodePath, iterator *js_ast.StmtIterator) {
			switch s.Data.(type) {
			case *js_ast.SExpr:
				if expr, ok := s.Data.(*js_ast.SExpr).Value.Data.(*js_ast.EIdentifier); ok {
					name := ast.Symbols[expr.Ref.InnerIndex].OriginalName
					if strings.HasPrefix(name, "$$") && strings.HasSuffix(name, "$$") {
						name = name[2 : len(name)-2]
						if node, ok := data[name]; ok {
							s.Data = node.(js_ast.S)
						}
					}
				}
			}
		},
		VisitExpr: func(e *js_ast.Expr, nodePath *js_ast.NodePath) {
			switch e.Data.(type) {
			case *js_ast.EIdentifier:
				name := ast.Symbols[e.Data.(*js_ast.EIdentifier).Ref.InnerIndex].OriginalName
				if strings.HasPrefix(name, "$$") && strings.HasSuffix(name, "$$") {
					name = name[2 : len(name)-2]
					if node, ok := data[name]; ok {
						e.Data = node.(js_ast.E)
					}
				}
			}
		},
	})
	return &ast
}
