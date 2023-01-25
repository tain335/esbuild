package js_ast

type ASTVisitorInterface interface {
	enterStmt(s *Stmt, path *NodePath, iterator *StmtIterator)
	exitStmt(s *Stmt, path *NodePath, iterator *StmtIterator)
	enterExpr(e *Expr, path *NodePath)
	exitExpr(e *Expr, path *NodePath)
}

type NodePath struct {
	ParentPath *NodePath
	Node       interface{}
}

type ASTVisitor struct {
	EnterStmt func(s *Stmt, path *NodePath, iterator *StmtIterator)
	ExitStmt  func(s *Stmt, path *NodePath, iterator *StmtIterator)
	EnterExpr func(e *Expr, path *NodePath)
	ExitExpr  func(e *Expr, path *NodePath)
}

func (visitor *ASTVisitor) enterStmt(s *Stmt, path *NodePath, iterator *StmtIterator) {
	if visitor.EnterStmt != nil {
		visitor.EnterStmt(s, path, iterator)
	}
}

func (visitor *ASTVisitor) enterExpr(e *Expr, path *NodePath) {
	if visitor.EnterExpr != nil {
		visitor.EnterExpr(e, path)
	}
}

func (visitor *ASTVisitor) exitStmt(s *Stmt, path *NodePath, iterator *StmtIterator) {
	if visitor.ExitStmt != nil {
		visitor.ExitStmt(s, path, iterator)
	}
}

func (visitor *ASTVisitor) exitExpr(e *Expr, path *NodePath) {
	if visitor.ExitExpr != nil {
		visitor.ExitExpr(e, path)
	}
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

func visitArgs(args []Arg, visitor ASTVisitorInterface, parentPath *NodePath, iterator *StmtIterator) {
	for i := range args {
		for j := range args[i].TSDecorators {
			visitExpr(&args[i].TSDecorators[j], visitor, parentPath, iterator)
		}
		visitExpr(&args[i].DefaultOrNil, visitor, parentPath, iterator)
	}
}

func visitProperties(properties []Property, visitor ASTVisitorInterface, parentPath *NodePath, iterator *StmtIterator) {
	for i := range properties {
		property := &properties[i]
		if property != nil {
			if property.ClassStaticBlock != nil && property.ClassStaticBlock.Block.Stmts != nil {
				iterator := newStmtIterator(&property.ClassStaticBlock.Block.Stmts)
				for {
					stmt, ok := iterator.Next()
					if ok {
						visitStmt(stmt, visitor, parentPath, &iterator)
					} else {
						break
					}
				}
			}
			visitExpr(&property.Key, visitor, parentPath, iterator)
			visitExpr(&property.ValueOrNil, visitor, parentPath, iterator)
			visitExpr(&property.InitializerOrNil, visitor, parentPath, iterator)
			for k := range property.TSDecorators {
				visitExpr(&property.TSDecorators[k], visitor, parentPath, iterator)
			}
		}
	}
}

func visitClass(class *Class, visitor ASTVisitorInterface, parentPath *NodePath, iterator *StmtIterator) {
	for i := range class.TSDecorators {
		visitExpr(&class.TSDecorators[i], visitor, parentPath, iterator)
	}
	visitExpr(&class.ExtendsOrNil, visitor, parentPath, iterator)
	visitProperties(class.Properties, visitor, parentPath, iterator)
}

func visitStmt(stmt *Stmt, visitor ASTVisitorInterface, parentPath *NodePath, iterator *StmtIterator) {
	if stmt == nil {
		return
	}
	stmtPath := &NodePath{ParentPath: parentPath, Node: stmt}
	visitor.enterStmt(stmt, stmtPath, iterator)
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
		visitExpr(&stmt.Data.(*SExportEquals).Value, visitor, stmtPath, iterator)
	case *SLazyExport:
		visitExpr(&stmt.Data.(*SLazyExport).Value, visitor, stmtPath, iterator)
	case *SExpr:
		visitExpr(&stmt.Data.(*SExpr).Value, visitor, stmtPath, iterator)
	case *SEnum:
		for i := range stmt.Data.(*SEnum).Values {
			visitExpr(&stmt.Data.(*SEnum).Values[i].ValueOrNil, visitor, stmtPath, iterator)
		}
	case *SNamespace:
		iterateStmts(&stmt.Data.(*SNamespace).Stmts)
	case *SFunction:
		visitArgs(stmt.Data.(*SFunction).Fn.Args, visitor, stmtPath, iterator)
		iterateStmts(&stmt.Data.(*SFunction).Fn.Body.Block.Stmts)
	case *SClass:
		visitClass(&stmt.Data.(*SClass).Class, visitor, stmtPath, iterator)
	case *SLabel:
		visitStmt(&stmt.Data.(*SLabel).Stmt, visitor, stmtPath, iterator)
	case *SFor:
		visitStmt(&stmt.Data.(*SFor).InitOrNil, visitor, stmtPath, iterator)
		visitExpr(&stmt.Data.(*SFor).TestOrNil, visitor, stmtPath, iterator)
		visitExpr(&stmt.Data.(*SFor).UpdateOrNil, visitor, stmtPath, iterator)
		visitStmt(&stmt.Data.(*SFor).Body, visitor, stmtPath, iterator)
	case *SForIn:
		visitStmt(&stmt.Data.(*SForIn).Init, visitor, stmtPath, iterator)
		visitExpr(&stmt.Data.(*SForIn).Value, visitor, stmtPath, iterator)
		visitStmt(&stmt.Data.(*SForIn).Body, visitor, stmtPath, iterator)
	case *SForOf:
		visitStmt(&stmt.Data.(*SForOf).Init, visitor, stmtPath, iterator)
		visitExpr(&stmt.Data.(*SForOf).Value, visitor, stmtPath, iterator)
		visitStmt(&stmt.Data.(*SForOf).Body, visitor, stmtPath, iterator)
	case *SDoWhile:
		visitExpr(&stmt.Data.(*SDoWhile).Test, visitor, stmtPath, iterator)
		visitStmt(&stmt.Data.(*SDoWhile).Body, visitor, stmtPath, iterator)
	case *SWhile:
		visitExpr(&stmt.Data.(*SWhile).Test, visitor, stmtPath, iterator)
		visitStmt(&stmt.Data.(*SWhile).Body, visitor, stmtPath, iterator)
	case *SWith:
		visitExpr(&stmt.Data.(*SWith).Value, visitor, stmtPath, iterator)
		visitStmt(&stmt.Data.(*SWith).Body, visitor, stmtPath, iterator)
	case *STry:
		iterateStmts(&stmt.Data.(*STry).Block.Stmts)
		if stmt.Data.(*STry).Catch != nil {
			iterateStmts(&stmt.Data.(*STry).Catch.Block.Stmts)
		}
		if stmt.Data.(*STry).Finally != nil {
			iterateStmts(&stmt.Data.(*STry).Finally.Block.Stmts)
		}
	case *SSwitch:
		visitExpr(&stmt.Data.(*SSwitch).Test, visitor, stmtPath, iterator)
		for i := range stmt.Data.(*SSwitch).Cases {
			c := stmt.Data.(*SSwitch).Cases[i]
			visitExpr(&c.ValueOrNil, visitor, stmtPath, iterator)
			iterateStmts(&c.Body)
			// for j := range c.Body {
			// 	visitStmt(&c.Body[j], visitor, stmtPath)
			// }
		}
	case *SReturn:
		visitExpr(&stmt.Data.(*SReturn).ValueOrNil, visitor, stmtPath, iterator)
	case *SThrow:
		visitExpr(&stmt.Data.(*SThrow).Value, visitor, stmtPath, iterator)
	case *SLocal:
		dels := stmt.Data.(*SLocal).Decls
		for i := range dels {
			visitExpr(&dels[i].ValueOrNil, visitor, stmtPath, iterator)
		}
	case *SIf:
		visitExpr(&stmt.Data.(*SIf).Test, visitor, stmtPath, iterator)
		visitStmt(&stmt.Data.(*SIf).Yes, visitor, stmtPath, iterator)
		visitStmt(&stmt.Data.(*SIf).NoOrNil, visitor, stmtPath, iterator)
	}
	visitor.exitStmt(stmt, stmtPath, iterator)
}

func visitExpr(expr *Expr, visitor ASTVisitorInterface, parentPath *NodePath, iterator *StmtIterator) {
	if expr == nil {
		return
	}
	exprPath := &NodePath{ParentPath: parentPath, Node: expr}
	visitor.enterExpr(expr, exprPath)
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
			visitExpr(&items[i], visitor, exprPath, iterator)
		}
	case *EUnary:
		visitExpr(&expr.Data.(*EUnary).Value, visitor, exprPath, iterator)
	case *EBinary:
		visitExpr(&expr.Data.(*EBinary).Left, visitor, exprPath, iterator)
		visitExpr(&expr.Data.(*EBinary).Right, visitor, exprPath, iterator)
	case *ENew:
		args := expr.Data.(*ENew).Args
		for i := range args {
			visitExpr(&args[i], visitor, exprPath, iterator)
		}
		visitExpr(&expr.Data.(*ENew).Target, visitor, exprPath, iterator)
	case *ECall:
		args := expr.Data.(*ECall).Args
		for i := range args {
			visitExpr(&args[i], visitor, exprPath, iterator)
		}
		visitExpr(&expr.Data.(*ECall).Target, visitor, exprPath, iterator)

	case *EDot:
		visitExpr(&expr.Data.(*EDot).Target, visitor, exprPath, iterator)
	case *EIndex:
		visitExpr(&expr.Data.(*EIndex).Target, visitor, exprPath, iterator)
		visitExpr(&expr.Data.(*EIndex).Index, visitor, exprPath, iterator)
	case *EArrow:
		visitArgs(expr.Data.(*EArrow).Args, visitor, exprPath, iterator)
		iterateStmts(&expr.Data.(*EArrow).Body.Block.Stmts)
	case *EFunction:
		visitArgs(expr.Data.(*EFunction).Fn.Args, visitor, exprPath, iterator)
		iterateStmts(&expr.Data.(*EFunction).Fn.Body.Block.Stmts)
	case *EClass:
		visitClass(&expr.Data.(*EClass).Class, visitor, exprPath, iterator)
	case *EJSXElement:
		visitExpr(&expr.Data.(*EJSXElement).TagOrNil, visitor, exprPath, iterator)
		visitProperties(expr.Data.(*EJSXElement).Properties, visitor, exprPath, iterator)
		children := expr.Data.(*EJSXElement).Children
		for i := range children {
			visitExpr(&children[i], visitor, exprPath, iterator)
		}
	case *ESpread:
		visitExpr(&expr.Data.(*ESpread).Value, visitor, exprPath, iterator)
	case *ETemplate:
		visitExpr(&expr.Data.(*ETemplate).TagOrNil, visitor, exprPath, iterator)
		parts := expr.Data.(*ETemplate).Parts
		for i := range parts {
			visitExpr(&parts[i].Value, visitor, exprPath, iterator)
		}
	case *EInlinedEnum:
		visitExpr(&expr.Data.(*EInlinedEnum).Value, visitor, exprPath, iterator)
	case *EAwait:
		visitExpr(&expr.Data.(*EAwait).Value, visitor, exprPath, iterator)
	case *EYield:
		visitExpr(&expr.Data.(*EYield).ValueOrNil, visitor, exprPath, iterator)
	case *EIf:
		visitExpr(&expr.Data.(*EIf).Test, visitor, exprPath, iterator)
		visitExpr(&expr.Data.(*EIf).Yes, visitor, exprPath, iterator)
		visitExpr(&expr.Data.(*EIf).No, visitor, exprPath, iterator)
	case *EImportCall:
		visitExpr(&expr.Data.(*EImportCall).Expr, visitor, exprPath, iterator)
		visitExpr(&expr.Data.(*EImportCall).OptionsOrNil, visitor, exprPath, iterator)
	case *EObject:
		props := expr.Data.(*EObject).Properties
		for i := range props {
			if props[i].ClassStaticBlock != nil {
				for j := range props[i].ClassStaticBlock.Block.Stmts {
					visitStmt(&props[i].ClassStaticBlock.Block.Stmts[j], visitor, exprPath, iterator)
				}
			}
			visitExpr(&props[i].Key, visitor, exprPath, iterator)
			visitExpr(&props[i].ValueOrNil, visitor, exprPath, iterator)
			visitExpr(&props[i].InitializerOrNil, visitor, exprPath, iterator)
			for k := range props[i].TSDecorators {
				visitExpr(&props[i].TSDecorators[k], visitor, exprPath, iterator)
			}
		}
	}
	visitor.enterExpr(expr, exprPath)
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
	}
}
