package main

import (
	"fmt"
	"reflect"
	"sort"
)

const (
	elemTypeMAP    = 1
	elemTypeArray  = 2
	elemTypeSimple = 3
)

type diffNode struct {
	key    string
	expect string
	actual string

	elemType int
	present  byte

	child []*diffNode
}

func genSpaceStr(num int) string {
	if num <= 0 {
		return ""
	}

	s := make([]rune, num)
	for i := 0; i < num; i++ {
		s[i] = ' '
	}

	return string(s)
}

func (d *diffNode) String(indent int) string {
	prefix := genSpaceStr(indent)

	sz := len(d.child)
	if sz == 0 {
		txt1 := ""
		txt2 := ""
		if d.present&1 != 0 {
			txt1 = fmt.Sprintf("%s-  %s: %s", prefix, d.key, d.expect)
		}
		if d.present&2 != 0 {
			txt2 = fmt.Sprintf("\n%s+  %s: %s", prefix, d.key, d.actual)
		}

		return txt1 + txt2
	}

	prefix3 := ""
	if d.present == 1 {
		prefix3 = "-"
	} else if d.present == 2 {
		prefix3 = "+"
	}

	lc := make([]string, 0, sz)
	prefix2 := genSpaceStr(len(d.key) + len(prefix3))

	brace1 := "["
	brace2 := "]"
	if d.elemType == elemTypeMAP {
		brace1 = "{"
		brace2 = "}"
	}

	l1 := fmt.Sprintf("%s  %s: %s", prefix+prefix3, d.key, brace1)
	ln := fmt.Sprintf("%s  %s  %s", prefix, prefix2, brace2)

	for i := 0; i < sz; i++ {
		c := d.child[i].String(indent + 2 + len(prefix2))
		lc = append(lc, c)
	}

	ret := l1 + "\n" + lc[0]
	for i := 1; i < len(lc); i++ {
		ret = ret + "\n" + lc[i]
	}

	ret = ret + "\n" + ln
	return ret
}

func diffMap(key string, ep, at map[string]interface{}) (*diffNode, error) {
	d := &diffNode{key: key, present: 3, elemType: elemTypeMAP}

	k1 := getSortedKeys(ep)
	k2 := getSortedKeys(at)

	for _, k := range k1 {
		var err error
		var c *diffNode
		if _, ok := at[k]; ok {
			c, err = diffAny(k, ep[k], at[k])
		} else {
			c, err = diffSingle(1, k, ep[k])
		}
		if err != nil {
			return nil, err
		}
		if c != nil {
			d.child = append(d.child, c)
		}
	}

	for _, k := range k2 {
		var err error
		var c *diffNode
		if _, ok := ep[k]; !ok {
			c, err = diffSingle(2, k, at[k])
		}
		if err != nil {
			return nil, err
		}
		if c != nil {
			d.child = append(d.child, c)
		}
	}

	if len(d.child) == 0 {
		return nil, nil
	}

	return d, nil
}

func diffArray(key string, ep, at []interface{}) (*diffNode, error) {
	sz := len(ep)
	d := &diffNode{key: key, present: 3}

	if sz > len(at) {
		sz = len(at)
	}

	for i := 0; i < sz; i++ {
		c, err := diffAny(fmt.Sprintf("%d", i), ep[i], at[i])
		if err != nil {
			return nil, err
		}

		if c != nil {
			d.child = append(d.child, c)
		}
	}

	for i := sz; i < len(ep); i++ {
		k := fmt.Sprintf("%d", i)
		c, err := diffSingle(1, k, ep[i])
		if err != nil {
			return nil, err
		}
		if c != nil {
			d.child = append(d.child, c)
		}
	}

	for i := sz; i < len(at); i++ {
		k := fmt.Sprintf("%d", i)
		c, err := diffSingle(2, k, at[i])
		if err != nil {
			return nil, err
		}
		if c != nil {
			d.child = append(d.child, c)
		}
	}

	if len(d.child) > 0 {
		return d, nil
	}

	return nil, nil
}

func diffSingle(presence byte, key string, n interface{}) (*diffNode, error) {
	d := &diffNode{key: key, present: presence}
	switch v := n.(type) {
	case []interface{}:
		i := 1
		d.elemType = elemTypeArray
		for _, vv := range v {
			id := fmt.Sprintf("%d", i)
			cd, err := diffSingle(presence, id, vv)
			if err != nil {
				return nil, err
			}
			i++
			d.child = append(d.child, cd)
		}
	case map[string]interface{}:
		d.elemType = elemTypeMAP
		k := getSortedKeys(v)
		for _, v2 := range k {
			if v3, ok := v[v2]; ok {
				cd, err := diffSingle(presence, v2, v3)
				if err != nil {
					return nil, err
				}
				d.child = append(d.child, cd)
			}
		}
	default:
		txt := fmt.Sprintf("%+v", n)
		if presence == 1 {
			d.expect = txt
		} else {
			d.actual = txt
		}
	}

	return d, nil
}

func diffAny(key string, ep, at interface{}) (*diffNode, error) {
	if reflect.TypeOf(ep) != reflect.TypeOf(at) {
		d := &diffNode{key: key, present: 3}

		expect, err1 := diffSingle(1, key, ep)
		if err1 != nil {
			return nil, err1
		}
		actual, err2 := diffSingle(2, key, at)
		if err2 != nil {
			return nil, err2
		}

		if expect != nil {
			d.child = append(d.child, expect)
		}
		if actual != nil {
			d.child = append(d.child, actual)
		}
		return d, nil
	}

	switch v := ep.(type) {
	case []interface{}:
		c, err := diffArray(key, v, at.([]interface{}))
		if err != nil {
			return nil, err
		}
		return c, err
	case map[string]interface{}:
		c, err := diffMap(key, v, at.(map[string]interface{}))
		if err != nil {
			return nil, err
		}
		return c, err
	default:
		d := &diffNode{key: key, present: 3}
		d.expect = fmt.Sprintf("%+v", ep)
		d.actual = fmt.Sprintf("%+v", at)
		if d.expect != d.actual {
			return d, nil
		}
		return nil, nil
	}
}

func getSortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
