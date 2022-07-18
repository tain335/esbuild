package js_ast

type ASTVisitorInterface interface {
	Stmt(s *Stmt, path *NodePath, iterator *StmtIterator)
	Expr(e *Expr, path *NodePath)
}

type NodePath struct {
	ParentPath *NodePath
	Node       interface{}
}

type ASTVisitor struct {
	VisitStmt func(s *Stmt, path *NodePath, iterator *StmtIterator)
	VisitExpr func(e *Expr, path *NodePath)
}

func (visitor *ASTVisitor) Stmt(s *Stmt, path *NodePath, iterator *StmtIterator) {
	visitor.VisitStmt(s, path, iterator)
}

func (visitor *ASTVisitor) Expr(e *Expr, path *NodePath) {
	visitor.VisitExpr(e, path)
}

func insert[K interface{}](a []K, index int, value K) ([]K, bool) {
	if index < 0 {
		return a, false
	}
	if len(a) == index { // nil or empty slice or after last element
		return append(a, value), true
	}
	a = append(a[:index+1], a[index:]...) // index < len(a)
	a[index] = value
	return a, true
}

type StmtIterator struct {
	stmts  *[]Stmt
	cursor int
}

func (iterator *StmtIterator) Next() (*Stmt, bool) {
	if iterator.cursor >= len(*iterator.stmts) || len(*iterator.stmts) == 0 {
		return &Stmt{}, false
	}

	stmt := &(*iterator.stmts)[iterator.cursor]
	iterator.cursor++
	return stmt, true
}

func (iterator *StmtIterator) InsertBefore(s Stmt) {
	if stmts, ok := insert(*iterator.stmts, iterator.cursor-1, s); ok {
		*iterator.stmts = stmts
		iterator.cursor++
	}
}

func (iterator StmtIterator) InsertAfter(s Stmt) {
	if stmts, ok := insert(*iterator.stmts, iterator.cursor, s); ok {
		*iterator.stmts = stmts
	}
}

func newStmtIterator(stmts *[]Stmt) StmtIterator {
	return StmtIterator{
		stmts: stmts,
	}
}

func visitArgs(args []Arg, visitor ASTVisitorInterface, parentPath *NodePath) {
	for i := range args {
		for j := range args[i].TSDecorators {
			visitExpr(&args[i].TSDecorators[j], visitor, parentPath)
		}
		visitExpr(&args[i].DefaultOrNil, visitor, parentPath)
	}
}

func visitProperties(properties []Property, visitor ASTVisitorInterface, parentPath *NodePath) {
	for i := range properties {
		property := &properties[i]
		iterator := newStmtIterator(&property.ClassStaticBlock.Block.Stmts)
		for {
			stmt, ok := iterator.Next()
			if ok {
				visitStmt(stmt, visitor, parentPath, &iterator)
			} else {
				break
			}
		}
		// for j := range property.ClassStaticBlock.Stmts {
		// 	visitStmt(&property.ClassStaticBlock.Stmts[j], visitor, parentPath)
		// }
		visitExpr(&property.Key, visitor, parentPath)
		visitExpr(&property.ValueOrNil, visitor, parentPath)
		visitExpr(&property.InitializerOrNil, visitor, parentPath)
		for k := range property.TSDecorators {
			visitExpr(&property.TSDecorators[k], visitor, parentPath)
		}
	}
}

func visitClass(class *Class, visitor ASTVisitorInterface, parentPath *NodePath) {
	for i := range class.TSDecorators {
		visitExpr(&class.TSDecorators[i], visitor, parentPath)
	}
	visitExpr(&class.ExtendsOrNil, visitor, parentPath)
	visitProperties(class.Properties, visitor, parentPath)
}

func visitStmt(stmt *Stmt, visitor ASTVisitorInterface, parentPath *NodePath, iterator *StmtIterator) {
	stmtPath := &NodePath{ParentPath: parentPath, Node: stmt}
	visitor.Stmt(stmt, stmtPath, iterator)
	iterateStmts := func(stmts *[]Stmt) {
		iterator := newStmtIterator(stmts)
		for {
			stmt, ok := iterator.Next()
			if ok {
				visitStmt(stmt, visitor, parentPath, &iterator)
			} else {
				break
			}
		}
	}
	switch stmt.Data.(type) {
	case *SBlock:
		iterateStmts(&stmt.Data.(*SBlock).Stmts)
		// for i := range stmt.Data.(*SBlock).Stmts {
		// 	visitStmt(&stmt.Data.(*SBlock).Stmts[i], visitor, stmtPath)
		// }
	case *SExportDefault:
		visitStmt(&stmt.Data.(*SExportDefault).Value, visitor, stmtPath, iterator)
	case *SExportEquals:
		visitExpr(&stmt.Data.(*SExportEquals).Value, visitor, stmtPath)
	case *SLazyExport:
		visitExpr(&stmt.Data.(*SLazyExport).Value, visitor, stmtPath)
	case *SExpr:
		visitExpr(&stmt.Data.(*SExpr).Value, visitor, stmtPath)
	case *SEnum:
		for i := range stmt.Data.(*SEnum).Values {
			visitExpr(&stmt.Data.(*SEnum).Values[i].ValueOrNil, visitor, stmtPath)
		}
	case *SNamespace:
		iterateStmts(&stmt.Data.(*SNamespace).Stmts)
		// for i := range stmt.Data.(*SNamespace).Stmts {
		// 	visitStmt(&stmt.Data.(*SNamespace).Stmts[i], visitor, stmtPath)
		// }
	case *SFunction:
		visitArgs(stmt.Data.(*SFunction).Fn.Args, visitor, stmtPath)
		iterateStmts(&stmt.Data.(*SFunction).Fn.Body.Block.Stmts)
		// for i := range body.Stmts {
		// 	visitStmt(&body.Stmts[i], visitor, stmtPath)
		// }
	case *SClass:
		visitClass(&stmt.Data.(*SClass).Class, visitor, stmtPath)
	case *SLabel:
		visitStmt(&stmt.Data.(*SLabel).Stmt, visitor, stmtPath, iterator)
	case *SFor:
		visitStmt(&stmt.Data.(*SFor).InitOrNil, visitor, stmtPath, iterator)
		visitExpr(&stmt.Data.(*SFor).TestOrNil, visitor, stmtPath)
		visitExpr(&stmt.Data.(*SFor).UpdateOrNil, visitor, stmtPath)
		visitStmt(&stmt.Data.(*SFor).Body, visitor, stmtPath, iterator)
	case *SForIn:
		visitStmt(&stmt.Data.(*SForIn).Init, visitor, stmtPath, iterator)
		visitExpr(&stmt.Data.(*SForIn).Value, visitor, stmtPath)
		visitStmt(&stmt.Data.(*SForIn).Body, visitor, stmtPath, iterator)
	case *SForOf:
		visitStmt(&stmt.Data.(*SForOf).Init, visitor, stmtPath, iterator)
		visitExpr(&stmt.Data.(*SForOf).Value, visitor, stmtPath)
		visitStmt(&stmt.Data.(*SForOf).Body, visitor, stmtPath, iterator)
	case *SDoWhile:
		visitExpr(&stmt.Data.(*SDoWhile).Test, visitor, stmtPath)
		visitStmt(&stmt.Data.(*SDoWhile).Body, visitor, stmtPath, iterator)
	case *SWhile:
		visitExpr(&stmt.Data.(*SWhile).Test, visitor, stmtPath)
		visitStmt(&stmt.Data.(*SWhile).Body, visitor, stmtPath, iterator)
	case *SWith:
		visitExpr(&stmt.Data.(*SWith).Value, visitor, stmtPath)
		visitStmt(&stmt.Data.(*SWith).Body, visitor, stmtPath, iterator)
	case *STry:
		iterateStmts(&stmt.Data.(*STry).Block.Stmts)
		// for i := range body {
		// 	visitStmt(&body[i], visitor, stmtPath)
		// }
		if stmt.Data.(*STry).Catch != nil {
			iterateStmts(&stmt.Data.(*STry).Catch.Block.Stmts)
			// for i := range body {
			// 	visitStmt(&body[i], visitor, stmtPath)
			// }
		}
		if stmt.Data.(*STry).Finally != nil {
			iterateStmts(&stmt.Data.(*STry).Finally.Block.Stmts)
			// for i := range body {
			// 	visitStmt(&body[i], visitor, stmtPath)
			// }
		}
	case *SSwitch:
		visitExpr(&stmt.Data.(*SSwitch).Test, visitor, stmtPath)
		for i := range stmt.Data.(*SSwitch).Cases {
			c := stmt.Data.(*SSwitch).Cases[i]
			visitExpr(&c.ValueOrNil, visitor, stmtPath)
			iterateStmts(&c.Body)
			// for j := range c.Body {
			// 	visitStmt(&c.Body[j], visitor, stmtPath)
			// }
		}
	case *SReturn:
		visitExpr(&stmt.Data.(*SReturn).ValueOrNil, visitor, stmtPath)
	case *SThrow:
		visitExpr(&stmt.Data.(*SThrow).Value, visitor, stmtPath)
	case *SLocal:
		dels := stmt.Data.(*SLocal).Decls
		for i := range dels {
			visitExpr(&dels[i].ValueOrNil, visitor, stmtPath)
		}
	case *SIf:
		visitExpr(&stmt.Data.(*SIf).Test, visitor, stmtPath)
		visitStmt(&stmt.Data.(*SIf).Yes, visitor, stmtPath, iterator)
		visitStmt(&stmt.Data.(*SIf).NoOrNil, visitor, stmtPath, iterator)
	}
}

func visitExpr(expr *Expr, visitor ASTVisitorInterface, parentPath *NodePath) {
	exprPath := &NodePath{ParentPath: parentPath, Node: expr}
	visitor.Expr(expr, exprPath)
	iterateStmts := func(stmts *[]Stmt) {
		iterator := newStmtIterator(stmts)
		for {
			stmt, ok := iterator.Next()
			if ok {
				visitStmt(stmt, visitor, exprPath, &iterator)
			} else {
				break
			}
		}
	}
	switch expr.Data.(type) {
	case *EArray:
		items := expr.Data.(*EArray).Items
		for i := range items {
			visitExpr(&items[i], visitor, exprPath)
		}
	case *EUnary:
		visitExpr(&expr.Data.(*EUnary).Value, visitor, exprPath)
	case *EBinary:
		visitExpr(&expr.Data.(*EBinary).Left, visitor, exprPath)
		visitExpr(&expr.Data.(*EBinary).Right, visitor, exprPath)
	case *ENew:
		args := expr.Data.(*ENew).Args
		for i := range args {
			visitExpr(&args[i], visitor, exprPath)
		}
		visitExpr(&expr.Data.(*ENew).Target, visitor, exprPath)
	case *ECall:
		args := expr.Data.(*ECall).Args
		for i := range args {
			visitExpr(&args[i], visitor, exprPath)
		}
		visitExpr(&expr.Data.(*ECall).Target, visitor, exprPath)

	case *EDot:
		visitExpr(&expr.Data.(*EDot).Target, visitor, exprPath)
	case *EIndex:
		visitExpr(&expr.Data.(*EIndex).Target, visitor, exprPath)
		visitExpr(&expr.Data.(*EIndex).Index, visitor, exprPath)
	case *EArrow:
		visitArgs(expr.Data.(*EArrow).Args, visitor, exprPath)
		iterateStmts(&expr.Data.(*EArrow).Body.Block.Stmts)
		// stmts := expr.Data.(*EArrow).Body.Stmts
		// for i := range stmts {
		// 	visitStmt(&stmts[i], visitor, exprPath)
		// }
	case *EFunction:
		visitArgs(expr.Data.(*EFunction).Fn.Args, visitor, exprPath)
		iterateStmts(&expr.Data.(*EFunction).Fn.Body.Block.Stmts)
		// stmts := expr.Data.(*EFunction).Fn.Body.Stmts
		// for i := range stmts {
		// 	visitStmt(&stmts[i], visitor, exprPath)
		// }
	case *EClass:
		visitClass(&expr.Data.(*EClass).Class, visitor, exprPath)
	case *EJSXElement:
		visitExpr(&expr.Data.(*EJSXElement).TagOrNil, visitor, exprPath)
		visitProperties(expr.Data.(*EJSXElement).Properties, visitor, exprPath)
		children := expr.Data.(*EJSXElement).Children
		for i := range children {
			visitExpr(&children[i], visitor, exprPath)
		}
	case *ESpread:
		visitExpr(&expr.Data.(*ESpread).Value, visitor, exprPath)
	case *ETemplate:
		visitExpr(&expr.Data.(*ETemplate).TagOrNil, visitor, exprPath)
		parts := expr.Data.(*ETemplate).Parts
		for i := range parts {
			visitExpr(&parts[i].Value, visitor, exprPath)
		}
	case *EInlinedEnum:
		visitExpr(&expr.Data.(*EInlinedEnum).Value, visitor, exprPath)
	case *EAwait:
		visitExpr(&expr.Data.(*EAwait).Value, visitor, exprPath)
	case *EYield:
		visitExpr(&expr.Data.(*EYield).ValueOrNil, visitor, exprPath)
	case *EIf:
		visitExpr(&expr.Data.(*EIf).Test, visitor, exprPath)
		visitExpr(&expr.Data.(*EIf).Yes, visitor, exprPath)
		visitExpr(&expr.Data.(*EIf).No, visitor, exprPath)
	case *EImportCall:
		visitExpr(&expr.Data.(*EImportCall).Expr, visitor, exprPath)
		visitExpr(&expr.Data.(*EImportCall).OptionsOrNil, visitor, exprPath)
	}

}

func TraverseAST(parts []Part, visitor ASTVisitorInterface) {
	for i := range parts {
		root := &NodePath{}
		iterator := newStmtIterator(&parts[i].Stmts)
		for {
			stmt, ok := iterator.Next()
			if ok {
				visitStmt(stmt, visitor, root, &iterator)
			} else {
				break
			}
		}
		// for j := range parts[i].Stmts {
		// 	visitStmt(&parts[i].Stmts[j], visitor, root)
		// }
	}
}
