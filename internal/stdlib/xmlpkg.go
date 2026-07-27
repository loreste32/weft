package stdlib

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageXML — XML parse/stringify (Python xml lite).
func packageXML() runtime.Value {
	p := pkg()

	set(p, "parse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("xml.parse(s)", "xml"), nil
		}
		node, err := parseXMLRoot(args[0].String())
		if err != nil {
			return errRes(err.Error(), "xml"), nil
		}
		return runtime.Ok(xmlNodeToValue(node)), nil
	}, 1)

	set(p, "stringify", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		s, err := stringifyXML(args[0])
		if err != nil {
			return errRes(err.Error(), "xml"), nil
		}
		return runtime.Str(s), nil
	}, 1)

	set(p, "escape", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(xmlEscape(args[0].String())), nil
	}, 1)

	set(p, "unescape", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(xmlUnescape(args[0].String())), nil
	}, 1)

	// xml.find(node, name) -> map|null  first descendant (or self) with name
	set(p, "find", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Null(), nil
		}
		if n, ok := xmlFind(args[0], args[1].String()); ok {
			return n, nil
		}
		return runtime.Null(), nil
	}, 2)

	// xml.findall(node, name) -> list  all descendants with name
	set(p, "findall", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.List(), nil
		}
		var out []runtime.Value
		xmlFindAll(args[0], args[1].String(), &out)
		return runtime.List(out...), nil
	}, 2)

	// xml.text(node) -> str
	set(p, "text", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(mapGetStr(args[0], "text", "")), nil
	}, 1)

	// xml.attr(node, key) -> str|null
	set(p, "attr", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Null(), nil
		}
		attrs, ok := mapGet(args[0], "attrs")
		if !ok || attrs.Kind != runtime.KindMap {
			return runtime.Null(), nil
		}
		if v, ok := mapGet(attrs, args[1].String()); ok {
			return v, nil
		}
		return runtime.Null(), nil
	}, 2)

	return p
}

func xmlFind(node runtime.Value, name string) (runtime.Value, bool) {
	if mapGetStr(node, "name", "") == name {
		return node, true
	}
	kids, ok := mapGet(node, "children")
	if !ok || kids.Kind != runtime.KindList {
		return runtime.Null(), false
	}
	for _, c := range kids.Obj.(*runtime.ListObj).Items {
		if n, ok := xmlFind(c, name); ok {
			return n, true
		}
	}
	return runtime.Null(), false
}

func xmlFindAll(node runtime.Value, name string, out *[]runtime.Value) {
	if mapGetStr(node, "name", "") == name {
		*out = append(*out, node)
	}
	kids, ok := mapGet(node, "children")
	if !ok || kids.Kind != runtime.KindList {
		return
	}
	for _, c := range kids.Obj.(*runtime.ListObj).Items {
		xmlFindAll(c, name, out)
	}
}

type xmlNode struct {
	Name     string
	Attrs    map[string]string
	Text     string
	Children []xmlNode
}

func parseXMLRoot(s string) (xmlNode, error) {
	dec := xml.NewDecoder(strings.NewReader(s))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return xmlNode{}, fmt.Errorf("no root element")
		}
		if err != nil {
			return xmlNode{}, err
		}
		if se, ok := tok.(xml.StartElement); ok {
			return parseXMLElement(dec, se)
		}
	}
}

func parseXMLElement(dec *xml.Decoder, start xml.StartElement) (xmlNode, error) {
	n := xmlNode{
		Name:  qname(start.Name),
		Attrs: map[string]string{},
	}
	for _, a := range start.Attr {
		n.Attrs[qname(a.Name)] = a.Value
	}
	var texts []string
	for {
		tok, err := dec.Token()
		if err != nil {
			return n, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			child, err := parseXMLElement(dec, t)
			if err != nil {
				return n, err
			}
			n.Children = append(n.Children, child)
		case xml.EndElement:
			n.Text = strings.TrimSpace(strings.Join(texts, " "))
			return n, nil
		case xml.CharData:
			if s := strings.TrimSpace(string(t)); s != "" {
				texts = append(texts, s)
			}
		}
	}
}

func qname(n xml.Name) string {
	if n.Space != "" {
		return n.Space + ":" + n.Local
	}
	return n.Local
}

func xmlNodeToValue(n xmlNode) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(k string, v runtime.Value) {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	put("name", runtime.Str(n.Name))
	put("text", runtime.Str(n.Text))
	attrs := runtime.NewMap()
	amo := attrs.Obj.(*runtime.MapObj)
	for k, v := range n.Attrs {
		amo.Keys = append(amo.Keys, k)
		amo.Vals[k] = runtime.Str(v)
	}
	put("attrs", attrs)
	kids := make([]runtime.Value, len(n.Children))
	for i, c := range n.Children {
		kids[i] = xmlNodeToValue(c)
	}
	put("children", runtime.List(kids...))
	return m
}

func stringifyXML(v runtime.Value) (string, error) {
	if v.Kind == runtime.KindStr {
		return v.String(), nil
	}
	if v.Kind != runtime.KindMap {
		return "", fmt.Errorf("xml.stringify expects map node or string")
	}
	var b strings.Builder
	writeXMLNode(&b, v)
	return b.String(), nil
}

func writeXMLNode(b *strings.Builder, v runtime.Value) {
	name := mapGetStr(v, "name", "item")
	text := mapGetStr(v, "text", "")
	b.WriteByte('<')
	b.WriteString(name)
	if attrs, ok := mapGet(v, "attrs"); ok && attrs.Kind == runtime.KindMap {
		amo := attrs.Obj.(*runtime.MapObj)
		seen := map[string]bool{}
		for _, k := range amo.Keys {
			seen[k] = true
			b.WriteByte(' ')
			b.WriteString(k)
			b.WriteString(`="`)
			b.WriteString(xmlEscape(amo.Vals[k].String()))
			b.WriteByte('"')
		}
		for k, val := range amo.Vals {
			if seen[k] {
				continue
			}
			b.WriteByte(' ')
			b.WriteString(k)
			b.WriteString(`="`)
			b.WriteString(xmlEscape(val.String()))
			b.WriteByte('"')
		}
	}
	children, _ := mapGet(v, "children")
	hasKids := children.Kind == runtime.KindList && len(children.Obj.(*runtime.ListObj).Items) > 0
	if !hasKids && text == "" {
		b.WriteString("/>")
		return
	}
	b.WriteByte('>')
	if text != "" {
		b.WriteString(xmlEscape(text))
	}
	if hasKids {
		for _, c := range children.Obj.(*runtime.ListObj).Items {
			writeXMLNode(b, c)
		}
	}
	b.WriteString("</")
	b.WriteString(name)
	b.WriteByte('>')
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func xmlUnescape(s string) string {
	replacer := strings.NewReplacer(
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&apos;", "'",
		"&amp;", "&",
	)
	return replacer.Replace(s)
}
