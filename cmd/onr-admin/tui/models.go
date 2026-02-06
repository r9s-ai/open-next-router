package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/r9s-ai/open-next-router/cmd/onr-admin/store"
	"github.com/r9s-ai/open-next-router/internal/models"
)

func (a *app) menuModels() error {
	for {
		ids := store.ListModelIDs(a.modelsDoc)
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "== models.yaml ==")
		if len(ids) == 0 {
			fmt.Fprintln(a.out, "(没有 model)")
		} else {
			for i, id := range ids {
				fmt.Fprintf(a.out, "%d) %s\n", i+1, id)
			}
		}
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "a) 新增 model")
		fmt.Fprintln(a.out, "e) 编辑 model")
		fmt.Fprintln(a.out, "d) 删除 model")
		fmt.Fprintln(a.out, "b) 返回")

		s, err := a.readLine("选择(数字) 或命令(a/e/d/b): ")
		if err != nil {
			return err
		}
		s = strings.ToLower(strings.TrimSpace(s))
		switch s {
		case "b":
			return nil
		case "a":
			if err := a.addModel(); err != nil {
				return err
			}
		case "e":
			if err := a.editModel(ids); err != nil {
				return err
			}
		case "d":
			if err := a.deleteModelEntry(ids); err != nil {
				return err
			}
		default:
			a.printModelByIndex(s, ids)
		}
	}
}

func (a *app) addModel() error {
	id, err := a.readLine("model id: ")
	if err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	if _, ok := store.GetModelNode(a.modelsDoc, id); ok {
		fmt.Fprintln(a.out, "已存在该 model。")
		return nil
	}

	provsIn, err := a.readLine("providers(逗号分隔，如 openai,anthropic): ")
	if err != nil {
		return err
	}
	provs := store.ParseProviders(provsIn)

	strategy, err := a.readLine("strategy(默认 round_robin): ")
	if err != nil {
		return err
	}
	strategy = strings.TrimSpace(strategy)
	if strategy == "" {
		strategy = string(models.StrategyRoundRobin)
	}

	ownedBy, err := a.readLine("owned_by(可空): ")
	if err != nil {
		return err
	}

	rt := models.Route{
		Providers: provs,
		Strategy:  models.Strategy(strings.TrimSpace(strategy)),
		OwnedBy:   strings.TrimSpace(ownedBy),
	}
	if err := store.SetModelRoute(a.modelsDoc, id, rt); err != nil {
		fmt.Fprintf(a.out, "新增失败: %v\n", err)
		return nil
	}
	a.modelsDirty = true
	return nil
}

func (a *app) editModel(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	n, err := a.readInt("输入要编辑的 model 序号: ")
	if err != nil {
		return err
	}
	if n <= 0 || n > len(ids) {
		return nil
	}

	id := ids[n-1]
	rt, _ := store.GetModelRoute(a.modelsDoc, id)
	fmt.Fprintf(a.out, "当前 providers=%q strategy=%q owned_by=%q\n", strings.Join(rt.Providers, ","), string(rt.Strategy), rt.OwnedBy)

	provsIn, err := a.readLine("新 providers(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}
	strategy, err := a.readLine("新 strategy(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}
	ownedBy, err := a.readLine("新 owned_by(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}

	up := store.ModelUpdate{}
	if strings.TrimSpace(provsIn) != "" {
		if strings.TrimSpace(provsIn) == "-" {
			up.Providers = store.Ptr([]string(nil))
		} else {
			up.Providers = store.Ptr(store.ParseProviders(provsIn))
		}
	}
	if strings.TrimSpace(strategy) != "" {
		if strings.TrimSpace(strategy) == "-" {
			up.Strategy = store.Ptr("")
		} else {
			up.Strategy = store.Ptr(strings.TrimSpace(strategy))
		}
	}
	if strings.TrimSpace(ownedBy) != "" {
		if strings.TrimSpace(ownedBy) == "-" {
			up.OwnedBy = store.Ptr("")
		} else {
			up.OwnedBy = store.Ptr(strings.TrimSpace(ownedBy))
		}
	}

	if err := store.UpdateModelRoute(a.modelsDoc, id, up); err != nil {
		fmt.Fprintf(a.out, "编辑失败: %v\n", err)
		return nil
	}
	a.modelsDirty = true
	return nil
}

func (a *app) deleteModelEntry(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	name, err := a.readLine("输入要删除的 model id: ")
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	ok, err := a.confirm("确认删除 model " + name + " ? (y/N): ")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if err := store.DeleteModel(a.modelsDoc, name); err != nil {
		fmt.Fprintf(a.out, "删除失败: %v\n", err)
		return nil
	}
	a.modelsDirty = true
	return nil
}

func (a *app) printModelByIndex(s string, ids []string) {
	// allow quick enter by index
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > len(ids) {
		return
	}
	id := ids[n-1]
	rt, _ := store.GetModelRoute(a.modelsDoc, id)
	fmt.Fprintln(a.out, "")
	fmt.Fprintf(a.out, "model=%s providers=%q strategy=%q owned_by=%q\n", id, strings.Join(rt.Providers, ","), string(rt.Strategy), rt.OwnedBy)
}
