package clause

import (
	"strings"
)

const (
	AndWithSpace = " AND "
	OrWithSpace  = " OR "
)

// Where 结构体，用于存储WHERE子句的表达式。
type Where struct {
	Exprs []Expression
}

// Name 获取WHERE子句的名称。
func (where Where) Name() string {
	return "WHERE"
}

// Build 构建WHERE子句的SQL。
func (where Where) Build(builder Builder) {
	if len(where.Exprs) == 1 {
		if andCondition, ok := where.Exprs[0].(AndConditions); ok {
			where.Exprs = andCondition.Exprs
		}
	}

	// 如果第一个查询表达式是单个Or条件，则交换位置。
	for idx, expr := range where.Exprs {
		if v, ok := expr.(OrConditions); !ok || len(v.Exprs) > 1 {
			if idx != 0 {
				where.Exprs[0], where.Exprs[idx] = where.Exprs[idx], where.Exprs[0]
			}
			break
		}
	}

	buildExprs(where.Exprs, builder, AndWithSpace)
}

// buildExprs 构建表达式。
func buildExprs(exprs []Expression, builder Builder, joinCond string) {
	wrapInParentheses := false

	for idx, expr := range exprs {
		if idx > 0 {
			if v, ok := expr.(OrConditions); ok && len(v.Exprs) == 1 {
				builder.WriteString(OrWithSpace)
			} else {
				builder.WriteString(joinCond)
			}
		}

		if len(exprs) > 1 {
			switch v := expr.(type) {
			case OrConditions:
				if len(v.Exprs) == 1 {
					if e, ok := v.Exprs[0].(Expr); ok {
						sql := strings.ToUpper(e.SQL)
						wrapInParentheses = strings.Contains(sql, AndWithSpace) || strings.Contains(sql, OrWithSpace)
					}
				}
			case AndConditions:
				if len(v.Exprs) == 1 {
					if e, ok := v.Exprs[0].(Expr); ok {
						sql := strings.ToUpper(e.SQL)
						wrapInParentheses = strings.Contains(sql, AndWithSpace) || strings.Contains(sql, OrWithSpace)
					}
				}
			case Expr:
				sql := strings.ToUpper(v.SQL)
				wrapInParentheses = strings.Contains(sql, AndWithSpace) || strings.Contains(sql, OrWithSpace)
			case NamedExpr:
				sql := strings.ToUpper(v.SQL)
				wrapInParentheses = strings.Contains(sql, AndWithSpace) || strings.Contains(sql, OrWithSpace)
			}
		}

		if wrapInParentheses {
			builder.WriteByte('(')
			expr.Build(builder)
			builder.WriteByte(')')
			wrapInParentheses = false
		} else {
			expr.Build(builder)
		}
	}
}

// MergeClause merge where clauses
func (where Where) MergeClause(clause *Clause) {
	if w, ok := clause.Expression.(Where); ok {
		exprs := make([]Expression, len(w.Exprs)+len(where.Exprs))
		copy(exprs, w.Exprs)
		copy(exprs[len(w.Exprs):], where.Exprs)
		where.Exprs = exprs
	}

	clause.Expression = where
}

// And 构建AND条件。
func And(exprs ...Expression) Expression {
	if len(exprs) == 0 {
		return nil
	}

	if len(exprs) == 1 {
		if _, ok := exprs[0].(OrConditions); !ok {
			return exprs[0]
		}
	}

	return AndConditions{Exprs: exprs}
}

// AndConditions 结构体，用于存储AND条件。
type AndConditions struct {
	Exprs []Expression
}

// Build 构建AND条件的SQL。
func (and AndConditions) Build(builder Builder) {
	if len(and.Exprs) > 1 {
		builder.WriteByte('(')
		buildExprs(and.Exprs, builder, AndWithSpace)
		builder.WriteByte(')')
	} else {
		buildExprs(and.Exprs, builder, AndWithSpace)
	}
}

// Or 构建OR条件。
func Or(exprs ...Expression) Expression {
	if len(exprs) == 0 {
		return nil
	}
	return OrConditions{Exprs: exprs}
}

// OrConditions 结构体，用于存储OR条件。
type OrConditions struct {
	Exprs []Expression
}

// Build 构建OR条件的SQL。
func (or OrConditions) Build(builder Builder) {
	if len(or.Exprs) > 1 {
		builder.WriteByte('(')
		buildExprs(or.Exprs, builder, OrWithSpace)
		builder.WriteByte(')')
	} else {
		buildExprs(or.Exprs, builder, OrWithSpace)
	}
}

// Not 构建NOT条件。
func Not(exprs ...Expression) Expression {
	if len(exprs) == 0 {
		return nil
	}
	if len(exprs) == 1 {
		if andCondition, ok := exprs[0].(AndConditions); ok {
			exprs = andCondition.Exprs
		}
	}
	return NotConditions{Exprs: exprs}
}

// NotConditions 结构体，用于存储NOT条件。
type NotConditions struct {
	Exprs []Expression
}

// Build 构建NOT条件的SQL。
func (not NotConditions) Build(builder Builder) {
	anyNegationBuilder := false
	for _, c := range not.Exprs {
		if _, ok := c.(NegationExpressionBuilder); ok {
			anyNegationBuilder = true
			break
		}
	}

	if anyNegationBuilder {
		if len(not.Exprs) > 1 {
			builder.WriteByte('(')
		}

		for idx, c := range not.Exprs {
			if idx > 0 {
				builder.WriteString(AndWithSpace)
			}

			if negationBuilder, ok := c.(NegationExpressionBuilder); ok {
				negationBuilder.NegationBuild(builder)
			} else {
				builder.WriteString("NOT ")
				e, wrapInParentheses := c.(Expr)
				if wrapInParentheses {
					sql := strings.ToUpper(e.SQL)
					if wrapInParentheses = strings.Contains(sql, AndWithSpace) || strings.Contains(sql, OrWithSpace); wrapInParentheses {
						builder.WriteByte('(')
					}
				}

				c.Build(builder)

				if wrapInParentheses {
					builder.WriteByte(')')
				}
			}
		}

		if len(not.Exprs) > 1 {
			builder.WriteByte(')')
		}
	} else {
		builder.WriteString("NOT ")
		if len(not.Exprs) > 1 {
			builder.WriteByte('(')
		}

		for idx, c := range not.Exprs {
			if idx > 0 {
				switch c.(type) {
				case OrConditions:
					builder.WriteString(OrWithSpace)
				default:
					builder.WriteString(AndWithSpace)
				}
			}

			e, wrapInParentheses := c.(Expr)
			if wrapInParentheses {
				sql := strings.ToUpper(e.SQL)
				if wrapInParentheses = strings.Contains(sql, AndWithSpace) || strings.Contains(sql, OrWithSpace); wrapInParentheses {
					builder.WriteByte('(')
				}
			}

			c.Build(builder)

			if wrapInParentheses {
				builder.WriteByte(')')
			}
		}

		if len(not.Exprs) > 1 {
			builder.WriteByte(')')
		}
	}
}
