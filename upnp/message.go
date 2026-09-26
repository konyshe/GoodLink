package upnp

import (
	"encoding/xml"
	"sort"
	"strings"
)

type Node struct {
	Name    string
	Content string
	Attr    map[string]string
	Child   []Node
}

func (n *Node) AddChild(node Node) {
	n.Child = append(n.Child, node)
}

func (n *Node) BuildXML() string {
	var buf strings.Builder
	n.writeXML(&buf)
	return buf.String()
}

func (n *Node) writeXML(buf *strings.Builder) {
	buf.WriteByte('<')
	buf.WriteString(n.Name)
	if len(n.Attr) > 0 {
		keys := make([]string, 0, len(n.Attr))
		for key := range n.Attr {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			buf.WriteByte(' ')
			buf.WriteString(key)
			buf.WriteByte('=')
			buf.WriteString(n.Attr[key])
		}
	}
	buf.WriteByte('>')
	// 必须转义: 端口映射描述等字段可能包含 & < >, 原样写出会生成畸形 SOAP,
	// 也会让外部可控数据闭合标签注入内容
	if err := xml.EscapeText(buf, []byte(n.Content)); err != nil {
		// EscapeText 只会在遇到非法码点时失败, 此时宁可放弃该内容也不能写出畸形报文
		buf.WriteString("")
	}
	for i := range n.Child {
		n.Child[i].writeXML(buf)
	}
	buf.WriteString("</")
	buf.WriteString(n.Name)
	buf.WriteByte('>')
}
