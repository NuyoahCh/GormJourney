package gorm

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"gorm.io/gorm/schema"
	"gorm.io/gorm/utils"
)

// 各种生命周期回调（如 create、update、delete、query 等）。

func initializeCallbacks(db *DB) *callbacks {
	return &callbacks{
		processors: map[string]*processor{
			"create": {db: db},
			"query":  {db: db},
			"update": {db: db},
			"delete": {db: db},
			"row":    {db: db},
			"raw":    {db: db},
		},
	}
}

// callbacks gorm callbacks manager
type callbacks struct {
	processors map[string]*processor
}

type processor struct {
	db        *DB
	Clauses   []string
	fns       []func(*DB)
	callbacks []*callback
}

type callback struct {
	name      string
	before    string
	after     string
	remove    bool
	replace   bool
	match     func(*DB) bool
	handler   func(*DB)
	processor *processor
}

// 创建回调。
func (cs *callbacks) Create() *processor {
	return cs.processors["create"]
}

// 查询回调。
func (cs *callbacks) Query() *processor {
	return cs.processors["query"]
}

// 更新回调。
func (cs *callbacks) Update() *processor {
	return cs.processors["update"]
}

// 删除回调。
func (cs *callbacks) Delete() *processor {
	return cs.processors["delete"]
}

// 行回调。
func (cs *callbacks) Row() *processor {
	return cs.processors["row"]
}

// 原始回调。
func (cs *callbacks) Raw() *processor {
	return cs.processors["raw"]
}

// 执行回调。
func (p *processor) Execute(db *DB) *DB {
	// call scopes
	for len(db.Statement.scopes) > 0 {
		db = db.executeScopes()
	}

	var (
		curTime           = time.Now()
		stmt              = db.Statement
		resetBuildClauses bool
	)

	if len(stmt.BuildClauses) == 0 {
		stmt.BuildClauses = p.Clauses
		resetBuildClauses = true
	}

	if optimizer, ok := db.Statement.Dest.(StatementModifier); ok {
		optimizer.ModifyStatement(stmt)
	}

	// assign model values
	if stmt.Model == nil {
		stmt.Model = stmt.Dest
	} else if stmt.Dest == nil {
		stmt.Dest = stmt.Model
	}

	// parse model values
	if stmt.Model != nil {
		if err := stmt.Parse(stmt.Model); err != nil && (!errors.Is(err, schema.ErrUnsupportedDataType) || (stmt.Table == "" && stmt.TableExpr == nil && stmt.SQL.Len() == 0)) {
			if errors.Is(err, schema.ErrUnsupportedDataType) && stmt.Table == "" && stmt.TableExpr == nil {
				db.AddError(fmt.Errorf("%w: Table not set, please set it like: db.Model(&user) or db.Table(\"users\")", err))
			} else {
				db.AddError(err)
			}
		}
	}

	// assign stmt.ReflectValue
	if stmt.Dest != nil {
		stmt.ReflectValue = reflect.ValueOf(stmt.Dest)
		for stmt.ReflectValue.Kind() == reflect.Ptr {
			if stmt.ReflectValue.IsNil() && stmt.ReflectValue.CanAddr() {
				stmt.ReflectValue.Set(reflect.New(stmt.ReflectValue.Type().Elem()))
			}

			stmt.ReflectValue = stmt.ReflectValue.Elem()
		}
		if !stmt.ReflectValue.IsValid() {
			db.AddError(ErrInvalidValue)
		}
	}

	for _, f := range p.fns {
		f(db)
	}

	if stmt.SQL.Len() > 0 {
		db.Logger.Trace(stmt.Context, curTime, func() (string, int64) {
			sql, vars := stmt.SQL.String(), stmt.Vars
			if filter, ok := db.Logger.(ParamsFilter); ok {
				sql, vars = filter.ParamsFilter(stmt.Context, stmt.SQL.String(), stmt.Vars...)
			}
			return db.Dialector.Explain(sql, vars...), db.RowsAffected
		}, db.Error)
	}

	if !stmt.DB.DryRun {
		stmt.SQL.Reset()
		stmt.Vars = nil
	}

	if resetBuildClauses {
		stmt.BuildClauses = nil
	}

	return db
}

// 获取回调。
func (p *processor) Get(name string) func(*DB) {
	for i := len(p.callbacks) - 1; i >= 0; i-- {
		if v := p.callbacks[i]; v.name == name && !v.remove {
			return v.handler
		}
	}
	return nil
}

// 在回调之前执行。
func (p *processor) Before(name string) *callback {
	return &callback{before: name, processor: p}
}

// 在回调之后执行。
func (p *processor) After(name string) *callback {
	return &callback{after: name, processor: p}
}

// 匹配回调。
func (p *processor) Match(fc func(*DB) bool) *callback {
	return &callback{match: fc, processor: p}
}

// 注册回调。
func (p *processor) Register(name string, fn func(*DB)) error {
	return (&callback{processor: p}).Register(name, fn)
}

// 删除回调。
func (p *processor) Remove(name string) error {
	return (&callback{processor: p}).Remove(name)
}

// 替换回调。
func (p *processor) Replace(name string, fn func(*DB)) error {
	return (&callback{processor: p}).Replace(name, fn)
}

// 编译回调。
func (p *processor) compile() (err error) {
	var callbacks []*callback
	removedMap := map[string]bool{}
	for _, callback := range p.callbacks {
		if callback.match == nil || callback.match(p.db) {
			callbacks = append(callbacks, callback)
		}
		if callback.remove {
			removedMap[callback.name] = true
		}
	}

	if len(removedMap) > 0 {
		callbacks = removeCallbacks(callbacks, removedMap)
	}
	p.callbacks = callbacks

	if p.fns, err = sortCallbacks(p.callbacks); err != nil {
		p.db.Logger.Error(context.Background(), "Got error when compile callbacks, got %v", err)
	}
	return
}

// 在回调之前执行。
func (c *callback) Before(name string) *callback {
	c.before = name
	return c
}

// 在回调之后执行。
func (c *callback) After(name string) *callback {
	c.after = name
	return c
}

// 注册回调。
func (c *callback) Register(name string, fn func(*DB)) error {
	c.name = name
	c.handler = fn
	c.processor.callbacks = append(c.processor.callbacks, c)
	return c.processor.compile()
}

// 删除回调。
func (c *callback) Remove(name string) error {
	c.processor.db.Logger.Warn(context.Background(), "removing callback `%s` from %s\n", name, utils.FileWithLineNum())
	c.name = name
	c.remove = true
	c.processor.callbacks = append(c.processor.callbacks, c)
	return c.processor.compile()
}

// 替换回调。
func (c *callback) Replace(name string, fn func(*DB)) error {
	c.processor.db.Logger.Info(context.Background(), "replacing callback `%s` from %s\n", name, utils.FileWithLineNum())
	c.name = name
	c.handler = fn
	c.replace = true
	c.processor.callbacks = append(c.processor.callbacks, c)
	return c.processor.compile()
}

// 获取右索引。
func getRIndex(strs []string, str string) int {
	for i := len(strs) - 1; i >= 0; i-- {
		if strs[i] == str {
			return i
		}
	}
	return -1
}

// 排序回调。
func sortCallbacks(cs []*callback) (fns []func(*DB), err error) {
	var (
		names, sorted []string
		sortCallback  func(*callback) error
	)
	sort.SliceStable(cs, func(i, j int) bool {
		if cs[j].before == "*" && cs[i].before != "*" {
			return true
		}
		if cs[j].after == "*" && cs[i].after != "*" {
			return true
		}
		return false
	})

	for _, c := range cs {
		// show warning message the callback name already exists
		if idx := getRIndex(names, c.name); idx > -1 && !c.replace && !c.remove && !cs[idx].remove {
			c.processor.db.Logger.Warn(context.Background(), "duplicated callback `%s` from %s\n", c.name, utils.FileWithLineNum())
		}
		names = append(names, c.name)
	}

	sortCallback = func(c *callback) error {
		if c.before != "" { // if defined before callback
			if c.before == "*" && len(sorted) > 0 {
				if curIdx := getRIndex(sorted, c.name); curIdx == -1 {
					sorted = append([]string{c.name}, sorted...)
				}
			} else if sortedIdx := getRIndex(sorted, c.before); sortedIdx != -1 {
				if curIdx := getRIndex(sorted, c.name); curIdx == -1 {
					// if before callback already sorted, append current callback just after it
					sorted = append(sorted[:sortedIdx], append([]string{c.name}, sorted[sortedIdx:]...)...)
				} else if curIdx > sortedIdx {
					return fmt.Errorf("conflicting callback %s with before %s", c.name, c.before)
				}
			} else if idx := getRIndex(names, c.before); idx != -1 {
				// if before callback exists
				cs[idx].after = c.name
			}
		}

		if c.after != "" { // if defined after callback
			if c.after == "*" && len(sorted) > 0 {
				if curIdx := getRIndex(sorted, c.name); curIdx == -1 {
					sorted = append(sorted, c.name)
				}
			} else if sortedIdx := getRIndex(sorted, c.after); sortedIdx != -1 {
				if curIdx := getRIndex(sorted, c.name); curIdx == -1 {
					// if after callback sorted, append current callback to last
					sorted = append(sorted, c.name)
				} else if curIdx < sortedIdx {
					return fmt.Errorf("conflicting callback %s with before %s", c.name, c.after)
				}
			} else if idx := getRIndex(names, c.after); idx != -1 {
				// if after callback exists but haven't sorted
				// set after callback's before callback to current callback
				after := cs[idx]

				if after.before == "" {
					after.before = c.name
				}

				if err := sortCallback(after); err != nil {
					return err
				}

				if err := sortCallback(c); err != nil {
					return err
				}
			}
		}

		// if current callback haven't been sorted, append it to last
		if getRIndex(sorted, c.name) == -1 {
			sorted = append(sorted, c.name)
		}

		return nil
	}

	for _, c := range cs {
		if err = sortCallback(c); err != nil {
			return
		}
	}

	for _, name := range sorted {
		if idx := getRIndex(names, name); !cs[idx].remove {
			fns = append(fns, cs[idx].handler)
		}
	}

	return
}

// 删除回调。
func removeCallbacks(cs []*callback, nameMap map[string]bool) []*callback {
	callbacks := make([]*callback, 0, len(cs))
	for _, callback := range cs {
		if nameMap[callback.name] {
			continue
		}
		callbacks = append(callbacks, callback)
	}
	return callbacks
}
