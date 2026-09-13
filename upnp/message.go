package upnp

import (
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
	buf.WriteString(n.Content)
	for i := range n.Child {
		n.Child[i].writeXML(buf)
	}
	buf.WriteString("</")
	buf.WriteString(n.Name)
	buf.WriteByte('>')
}
