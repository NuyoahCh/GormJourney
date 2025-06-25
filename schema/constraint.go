package schema

import (
	"regexp"
	"strings"

	"gorm.io/gorm/clause"
)

// reg match english letters and midline
var regEnLetterAndMidline = regexp.MustCompile(`^[\w-]+$`)

// CheckConstraint 结构体，用于存储检查约束相关的信息。
type CheckConstraint struct {
	Name       string
	Constraint string // length(phone) >= 10
	*Field
}

// GetName 获取检查约束的名称。
func (chk *CheckConstraint) GetName() string { return chk.Name }

// Build 构建检查约束的SQL。
func (chk *CheckConstraint) Build() (sql string, vars []interface{}) {
	return "CONSTRAINT ? CHECK (?)", []interface{}{clause.Column{Name: chk.Name}, clause.Expr{SQL: chk.Constraint}}
}

// ParseCheckConstraints 解析模式中的检查约束。
func (schema *Schema) ParseCheckConstraints() map[string]CheckConstraint {
	checks := map[string]CheckConstraint{}
	for _, field := range schema.FieldsByDBName {
		if chk := field.TagSettings["CHECK"]; chk != "" {
			names := strings.Split(chk, ",")
			if len(names) > 1 && regEnLetterAndMidline.MatchString(names[0]) {
				checks[names[0]] = CheckConstraint{Name: names[0], Constraint: strings.Join(names[1:], ","), Field: field}
			} else {
				if names[0] == "" {
					chk = strings.Join(names[1:], ",")
				}
				name := schema.namer.CheckerName(schema.Table, field.DBName)
				checks[name] = CheckConstraint{Name: name, Constraint: chk, Field: field}
			}
		}
	}
	return checks
}

// UniqueConstraint 结构体，用于存储唯一约束相关的信息。
type UniqueConstraint struct {
	Name  string
	Field *Field
}

// GetName 获取唯一约束的名称。
func (uni *UniqueConstraint) GetName() string { return uni.Name }

// Build 构建唯一约束的SQL。
func (uni *UniqueConstraint) Build() (sql string, vars []interface{}) {
	return "CONSTRAINT ? UNIQUE (?)", []interface{}{clause.Column{Name: uni.Name}, clause.Column{Name: uni.Field.DBName}}
}

// ParseUniqueConstraints 解析模式中的唯一约束。
func (schema *Schema) ParseUniqueConstraints() map[string]UniqueConstraint {
	uniques := make(map[string]UniqueConstraint)
	for _, field := range schema.Fields {
		if field.Unique {
			name := schema.namer.UniqueName(schema.Table, field.DBName)
			uniques[name] = UniqueConstraint{Name: name, Field: field}
		}
	}
	return uniques
}
