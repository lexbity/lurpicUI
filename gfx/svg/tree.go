package svg

import (
	"encoding/xml"
	"errors"
	"io"
)

func parseSVGTree(r io.Reader) (*svgNode, error) {
	decoder := xml.NewDecoder(r)
	var stack []*svgNode
	var root *svgNode
	for {
		tok, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			node := &svgNode{
				Name:  tok.Name.Local,
				Attrs: make(map[string]string, len(tok.Attr)),
			}
			for _, attr := range tok.Attr {
				node.Attrs[attr.Name.Local] = attr.Value
			}
			if len(stack) == 0 {
				root = node
			} else {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, node)
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, errors.New("svg: unexpected closing tag")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData, xml.Comment, xml.Directive:
		default:
		}
	}
	if root == nil {
		return nil, errors.New("svg: empty document")
	}
	return root, nil
}

func indexSVGNodes(node *svgNode, index map[string]*svgNode) {
	if node == nil {
		return
	}
	type indexFrame struct {
		node *svgNode
	}
	stack := []indexFrame{{node: node}}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if frame.node == nil {
			continue
		}
		if id := frame.node.Attrs["id"]; id != "" {
			index[id] = frame.node
		}
		for i := len(frame.node.Children) - 1; i >= 0; i-- {
			stack = append(stack, indexFrame{node: frame.node.Children[i]})
		}
	}
}
