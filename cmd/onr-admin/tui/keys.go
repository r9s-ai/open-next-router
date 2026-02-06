package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/r9s-ai/open-next-router/cmd/onr-admin/store"
	"github.com/r9s-ai/open-next-router/internal/keystore"
)

func (a *app) menuKeys() error {
	for {
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "== keys.yaml ==")
		fmt.Fprintln(a.out, "1) providers (上游 key 池)")
		fmt.Fprintln(a.out, "2) access_keys (访问 key 池)")
		fmt.Fprintln(a.out, "3) 返回")

		choice, err := a.readChoice(1, 3)
		if err != nil {
			return err
		}
		switch choice {
		case 1:
			if err := a.menuProviders(); err != nil {
				return err
			}
		case 2:
			if err := a.menuAccessKeys(); err != nil {
				return err
			}
		case 3:
			return nil
		}
	}
}
func (a *app) menuProviders() error {
	for {
		provs := store.ListProviders(a.keysDoc)
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "== providers (上游 key 池) ==")
		if len(provs) == 0 {
			fmt.Fprintln(a.out, "(没有 provider)")
		} else {
			for i, p := range provs {
				fmt.Fprintf(a.out, "%d) %s\n", i+1, p)
			}
		}
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "a) 新增 provider")
		fmt.Fprintln(a.out, "d) 删除 provider")
		fmt.Fprintln(a.out, "b) 返回")

		s, err := a.readLine("选择 provider(数字) 或命令(a/d/b): ")
		if err != nil {
			return err
		}
		s = strings.ToLower(strings.TrimSpace(s))
		switch s {
		case "b":
			return nil
		case "a":
			name, err := a.readLine("输入 provider 名称: ")
			if err != nil {
				return err
			}
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" {
				continue
			}
			if _, ok := store.GetProviderNode(a.keysDoc, name); ok {
				fmt.Fprintln(a.out, "已存在该 provider。")
				continue
			}
			store.EnsureProviderNode(a.keysDoc, name)
			a.keysDirty = true
		case "d":
			if len(provs) == 0 {
				continue
			}
			name, err := a.readLine("输入要删除的 provider 名称: ")
			if err != nil {
				return err
			}
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" {
				continue
			}
			ok, err := a.confirm("确认删除 provider " + name + " ? (y/N): ")
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			if err := store.DeleteProvider(a.keysDoc, name); err != nil {
				fmt.Fprintf(a.out, "删除失败: %v\n", err)
				continue
			}
			a.keysDirty = true
		default:
			n, err := strconv.Atoi(s)
			if err != nil || n <= 0 || n > len(provs) {
				continue
			}
			p := provs[n-1]
			if err := a.menuProvider(p); err != nil {
				return err
			}
		}
	}
}

func (a *app) menuAccessKeys() error {
	for {
		aks, _ := store.ListAccessKeysDoc(a.keysDoc)
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "== access_keys (访问 key 池) ==")
		if len(aks) == 0 {
			fmt.Fprintln(a.out, "(没有 access_key)")
		} else {
			for i, ak := range aks {
				name := strings.TrimSpace(ak.Name)
				if name == "" {
					name = fmt.Sprintf("#%d", i+1)
				}
				disabled := ""
				if ak.Disabled {
					disabled = " disabled=true"
				}
				c := strings.TrimSpace(ak.Comment)
				if c != "" {
					c = " comment=" + strconv.Quote(c)
				}
				fmt.Fprintf(a.out, "%d) name=%q value=%s%s%s\n", i+1, name, store.ValueHint(ak.Value), disabled, c)
			}
		}
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "1) 新增 access_key")
		fmt.Fprintln(a.out, "2) 编辑 access_key")
		fmt.Fprintln(a.out, "3) 删除 access_key")
		fmt.Fprintln(a.out, "4) 环境变量覆盖提示")
		fmt.Fprintln(a.out, "5) 返回")

		choice, err := a.readChoice(1, 5)
		if err != nil {
			return err
		}
		switch choice {
		case 1:
			if err := a.addAccessKey(); err != nil {
				return err
			}
		case 2:
			if err := a.editAccessKey(); err != nil {
				return err
			}
		case 3:
			if err := a.deleteAccessKey(); err != nil {
				return err
			}
		case 4:
			a.printAccessKeyEnvHints()
		case 5:
			return nil
		}
	}
}

func (a *app) addAccessKey() error {
	name, err := a.readLine("name(建议填写): ")
	if err != nil {
		return err
	}
	val, err := a.readLine("value(可空；支持明文或 ENC[...]): ")
	if err != nil {
		return err
	}
	disabledStr, err := a.readLine("disabled(y/N): ")
	if err != nil {
		return err
	}
	comment, err := a.readLine("comment(可空): ")
	if err != nil {
		return err
	}

	disabled := false
	switch strings.ToLower(strings.TrimSpace(disabledStr)) {
	case "y", "yes", "true", "1":
		disabled = true
	}

	val = strings.TrimSpace(val)
	if val != "" {
		enc, err := a.confirm("要把 value 加密写入 keys.yaml 吗？(y/N): ")
		if err != nil {
			return err
		}
		if enc {
			out, err := keystore.Encrypt(val)
			if err != nil {
				fmt.Fprintf(a.out, "加密失败: %v\n", err)
				return nil
			}
			val = out
		}
	}

	if err := store.AppendAccessKeyDoc(a.keysDoc, keystore.AccessKey{
		Name:     strings.TrimSpace(name),
		Value:    val,
		Disabled: disabled,
		Comment:  strings.TrimSpace(comment),
	}); err != nil {
		fmt.Fprintf(a.out, "新增失败: %v\n", err)
		return nil
	}
	a.keysDirty = true
	return nil
}

func (a *app) editAccessKey() error {
	aks, _ := store.ListAccessKeysDoc(a.keysDoc)
	if len(aks) == 0 {
		return nil
	}
	idx, err := a.readInt("输入要编辑的 access_key 序号: ")
	if err != nil {
		return err
	}
	if idx <= 0 || idx > len(aks) {
		return nil
	}
	cur := aks[idx-1]
	fmt.Fprintf(a.out, "当前 name=%q value=%s disabled=%v comment=%q\n", cur.Name, store.ValueHint(cur.Value), cur.Disabled, cur.Comment)

	name, err := a.readLine("新 name(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}
	val, err := a.readLine("新 value(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}
	disabledStr, err := a.readLine("新 disabled(y/N；留空=不改): ")
	if err != nil {
		return err
	}
	comment, err := a.readLine("新 comment(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}

	up := store.AccessKeyUpdate{}
	if strings.TrimSpace(name) != "" {
		if strings.TrimSpace(name) == "-" {
			up.Name = store.Ptr("")
		} else {
			up.Name = store.Ptr(strings.TrimSpace(name))
		}
	}
	if strings.TrimSpace(val) != "" {
		if strings.TrimSpace(val) == "-" {
			up.Value = store.Ptr("")
		} else {
			v := strings.TrimSpace(val)
			enc, err := a.confirm("要把 value 加密写入 keys.yaml 吗？(y/N): ")
			if err != nil {
				return err
			}
			if enc && v != "" {
				out, err := keystore.Encrypt(v)
				if err != nil {
					fmt.Fprintf(a.out, "加密失败: %v\n", err)
					return nil
				}
				v = out
			}
			up.Value = store.Ptr(v)
		}
	}
	if strings.TrimSpace(disabledStr) != "" {
		switch strings.ToLower(strings.TrimSpace(disabledStr)) {
		case "y", "yes", "true", "1":
			up.Disabled = store.Ptr(true)
		default:
			up.Disabled = store.Ptr(false)
		}
	}
	if strings.TrimSpace(comment) != "" {
		if strings.TrimSpace(comment) == "-" {
			up.Comment = store.Ptr("")
		} else {
			up.Comment = store.Ptr(strings.TrimSpace(comment))
		}
	}

	if err := store.UpdateAccessKeyDoc(a.keysDoc, idx-1, up); err != nil {
		fmt.Fprintf(a.out, "编辑失败: %v\n", err)
		return nil
	}
	a.keysDirty = true
	return nil
}

func (a *app) deleteAccessKey() error {
	aks, _ := store.ListAccessKeysDoc(a.keysDoc)
	if len(aks) == 0 {
		return nil
	}
	idx, err := a.readInt("输入要删除的 access_key 序号: ")
	if err != nil {
		return err
	}
	if idx <= 0 || idx > len(aks) {
		return nil
	}
	ok, err := a.confirm("确认删除该 access_key ? (y/N): ")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if err := store.DeleteAccessKeyDoc(a.keysDoc, idx-1); err != nil {
		fmt.Fprintf(a.out, "删除失败: %v\n", err)
		return nil
	}
	a.keysDirty = true
	return nil
}

func (a *app) printAccessKeyEnvHints() {
	aks, _ := store.ListAccessKeysDoc(a.keysDoc)
	fmt.Fprintln(a.out, "")
	fmt.Fprintln(a.out, "== env 覆盖提示: access_keys ==")
	if len(aks) == 0 {
		fmt.Fprintln(a.out, "(没有 access_key)")
		return
	}
	for i, ak := range aks {
		fmt.Fprintf(a.out, "%d) %s\n", i+1, store.AccessEnvVar(strings.TrimSpace(ak.Name), i))
	}
}

func (a *app) menuProvider(provider string) error {
	for {
		keys, _ := store.ListProviderKeys(a.keysDoc, provider)
		fmt.Fprintln(a.out, "")
		fmt.Fprintf(a.out, "== provider: %s ==\n", provider)
		if len(keys) == 0 {
			fmt.Fprintln(a.out, "(没有 key)")
		} else {
			for i, k := range keys {
				valHint := store.ValueHint(k.Value)
				bu := strings.TrimSpace(k.BaseURLOverride)
				if bu != "" {
					bu = " base_url_override=" + bu
				}
				fmt.Fprintf(a.out, "%d) name=%q value=%s%s\n", i+1, strings.TrimSpace(k.Name), valHint, bu)
			}
		}
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "1) 新增 key")
		fmt.Fprintln(a.out, "2) 编辑 key")
		fmt.Fprintln(a.out, "3) 删除 key")
		fmt.Fprintln(a.out, "4) 环境变量覆盖提示")
		fmt.Fprintln(a.out, "5) 返回")

		choice, err := a.readChoice(1, 5)
		if err != nil {
			return err
		}
		switch choice {
		case 1:
			if err := a.addKey(provider); err != nil {
				return err
			}
		case 2:
			if err := a.editKey(provider); err != nil {
				return err
			}
		case 3:
			if err := a.deleteKey(provider); err != nil {
				return err
			}
		case 4:
			a.printEnvHints(provider)
		case 5:
			return nil
		}
	}
}

func (a *app) addKey(provider string) error {
	name, err := a.readLine("name(可空): ")
	if err != nil {
		return err
	}
	val, err := a.readLine("value(可空；支持明文或 ENC[...]): ")
	if err != nil {
		return err
	}
	bu, err := a.readLine("base_url_override(可空): ")
	if err != nil {
		return err
	}

	val = strings.TrimSpace(val)
	if val != "" {
		enc, err := a.confirm("要把 value 加密写入 keys.yaml 吗？(y/N): ")
		if err != nil {
			return err
		}
		if enc {
			out, err := keystore.Encrypt(val)
			if err != nil {
				fmt.Fprintf(a.out, "加密失败: %v\n", err)
				return nil
			}
			val = out
		}
	}

	if err := store.AppendProviderKey(a.keysDoc, provider, keystore.Key{
		Name:            strings.TrimSpace(name),
		Value:           val,
		BaseURLOverride: strings.TrimSpace(bu),
	}); err != nil {
		fmt.Fprintf(a.out, "新增失败: %v\n", err)
		return nil
	}
	a.keysDirty = true
	return nil
}

func (a *app) editKey(provider string) error {
	keys, _ := store.ListProviderKeys(a.keysDoc, provider)
	if len(keys) == 0 {
		return nil
	}
	idx, err := a.readInt("输入要编辑的 key 序号: ")
	if err != nil {
		return err
	}
	if idx <= 0 || idx > len(keys) {
		return nil
	}
	cur := keys[idx-1]
	fmt.Fprintf(a.out, "当前 name=%q value=%s base_url_override=%q\n", cur.Name, store.ValueHint(cur.Value), cur.BaseURLOverride)

	name, err := a.readLine("新 name(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}
	val, err := a.readLine("新 value(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}
	bu, err := a.readLine("新 base_url_override(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}

	update := store.KeyUpdate{}
	if strings.TrimSpace(name) != "" {
		if strings.TrimSpace(name) == "-" {
			update.Name = store.Ptr("")
		} else {
			update.Name = store.Ptr(strings.TrimSpace(name))
		}
	}
	if strings.TrimSpace(val) != "" {
		if strings.TrimSpace(val) == "-" {
			update.Value = store.Ptr("")
		} else {
			v := strings.TrimSpace(val)
			enc, err := a.confirm("要把 value 加密写入 keys.yaml 吗？(y/N): ")
			if err != nil {
				return err
			}
			if enc && v != "" {
				out, err := keystore.Encrypt(v)
				if err != nil {
					fmt.Fprintf(a.out, "加密失败: %v\n", err)
					return nil
				}
				v = out
			}
			update.Value = store.Ptr(v)
		}
	}
	if strings.TrimSpace(bu) != "" {
		if strings.TrimSpace(bu) == "-" {
			update.BaseURLOverride = store.Ptr("")
		} else {
			update.BaseURLOverride = store.Ptr(strings.TrimSpace(bu))
		}
	}

	if err := store.UpdateProviderKey(a.keysDoc, provider, idx-1, update); err != nil {
		fmt.Fprintf(a.out, "编辑失败: %v\n", err)
		return nil
	}
	a.keysDirty = true
	return nil
}

func (a *app) deleteKey(provider string) error {
	keys, _ := store.ListProviderKeys(a.keysDoc, provider)
	if len(keys) == 0 {
		return nil
	}
	idx, err := a.readInt("输入要删除的 key 序号: ")
	if err != nil {
		return err
	}
	if idx <= 0 || idx > len(keys) {
		return nil
	}
	ok, err := a.confirm("确认删除该 key ? (y/N): ")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if err := store.DeleteProviderKey(a.keysDoc, provider, idx-1); err != nil {
		fmt.Fprintf(a.out, "删除失败: %v\n", err)
		return nil
	}
	a.keysDirty = true
	return nil
}

func (a *app) printEnvHints(provider string) {
	keys, _ := store.ListProviderKeys(a.keysDoc, provider)
	fmt.Fprintln(a.out, "")
	fmt.Fprintf(a.out, "== env 覆盖提示: provider=%s ==\n", provider)
	if len(keys) == 0 {
		fmt.Fprintln(a.out, "(没有 key)")
		return
	}
	for i, k := range keys {
		fmt.Fprintf(a.out, "%d) %s\n", i+1, store.UpstreamEnvVar(provider, strings.TrimSpace(k.Name), i))
	}
}
