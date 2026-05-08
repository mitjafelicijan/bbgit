package main

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type alertType struct {
	kind  string
	title string
	icon  string
}

var alertTypes = map[string]alertType{
	"NOTE": {
		kind:  "note",
		title: "Note",
		icon:  `<svg version="1.1" width="16" height="16" viewBox="0 0 16 16" class="octicon octicon-info" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" fill="currentColor"><path fill-rule="evenodd" d="M8 1.5a6.5 6.5 0 100 13 6.5 6.5 0 000-13zM0 8a8 8 0 1116 0A8 8 0 010 8zm6.5-.25A.75.75 0 017.25 7h1a.75.75 0 01.75.75v2.75h.25a.75.75 0 010 1.5h-2a.75.75 0 010-1.5h.25v-2h-.25a.75.75 0 01-.75-.75zM8 6a1 1 0 100-2 1 1 0 000 2z"/></svg>`,
	},
	"TIP": {
		kind:  "tip",
		title: "Tip",
		icon:  `<svg version="1.1" width="16" height="16" viewBox="0 0 16 16" class="octicon octicon-light-bulb" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" fill="currentColor"><path fill-rule="evenodd" d="M8 1.5c-2.363 0-4 1.69-4 3.75 0 .984.424 1.625.984 2.304l.214.253c.223.264.47.556.673.848.284.411.537.896.621 1.49a.75.75 0 01-1.484.211c-.04-.282-.163-.547-.37-.847a8.695 8.695 0 00-.542-.68c-.084-.1-.173-.205-.268-.32C3.201 7.75 2.5 6.766 2.5 5.25 2.5 2.31 4.863 0 8 0s5.5 2.31 5.5 5.25c0 1.516-.701 2.5-1.328 3.259-.095.115-.184.22-.268.319-.207.245-.383.453-.541.681-.208.3-.33.565-.37.847a.75.75 0 01-1.485-.212c.084-.593.337-1.078.621-1.489.203-.292.45-.584.673-.848.075-.088.147-.173.213-.253.561-.679.985-1.32.985-2.304 0-2.06-1.637-3.75-4-3.75zM6 15.25a.75.75 0 01.75-.75h2.5a.75.75 0 010 1.5h-2.5a.75.75 0 01-.75-.75zM5.75 12a.75.75 0 000 1.5h4.5a.75.75 0 000-1.5h-4.5z"/></svg>`,
	},
	"IMPORTANT": {
		kind:  "important",
		title: "Important",
		icon:  `<svg version="1.1" width="16" height="16" viewBox="0 0 16 16" class="octicon octicon-report" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" fill="currentColor"><path fill-rule="evenodd" d="M1.75 1.5a.25.25 0 00-.25.25v9.5c0 .138.112.25.25.25h2a.75.75 0 01.75.75v2.19l2.72-2.72a.75.75 0 01.53-.22h6.5a.25.25 0 00.25-.25v-9.5a.25.25 0 00-.25-.25H1.75zM0 1.75C0 .784.784 0 1.75 0h12.5C15.216 0 16 .784 16 1.75v9.5A1.75 1.75 0 0114.25 13H8.06l-2.573 2.573A1.457 1.457 0 013 14.543V13H1.75A1.75 1.75 0 010 11.25v-9.5zM9 9a1 1 0 11-2 0 1 1 0 012 0zm-.25-5.25a.75.75 0 00-1.5 0v2.5a.75.75 0 001.5 0v-2.5z"/></svg>`,
	},
	"WARNING": {
		kind:  "warning",
		title: "Warning",
		icon:  `<svg version="1.1" width="16" height="16" viewBox="0 0 16 16" class="octicon octicon-alert" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" fill="currentColor"><path fill-rule="evenodd" d="M8.22 1.754a.25.25 0 00-.44 0L1.698 13.132a.25.25 0 00.22.368h12.164a.25.25 0 00.22-.368L8.22 1.754zm-1.763-.707c.659-1.234 2.427-1.234 3.086 0l6.082 11.378A1.75 1.75 0 0114.082 15H1.918a1.75 1.75 0 01-1.543-2.575L6.457 1.047zM9 11a1 1 0 11-2 0 1 1 0 012 0zm-.25-5.25a.75.75 0 00-1.5 0v2.5a.75.75 0 001.5 0v-2.5z"/></svg>`,
	},
	"CAUTION": {
		kind:  "caution",
		title: "Caution",
		icon:  `<svg version="1.1" width="16" height="16" viewBox="0 0 16 16" class="octicon octicon-law" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" fill="currentColor"><path fill-rule="evenodd" d="M8.75.75a.75.75 0 00-1.5 0V2h-.984c-.305 0-.604.08-.869.23l-1.288.737A.25.25 0 013.984 3H1.75a.75.75 0 000 1.5h.428L.066 9.192a.75.75 0 00.154.838l.53-.53-.53.53v.001l.002.002.002.002.006.006.016.015.045.04a3.514 3.514 0 00.686.45A4.492 4.492 0 003 11c.88 0 1.556-.22 2.023-.454a3.515 3.515 0 00.686-.45l.045-.04.016-.015.006-.006.002-.002.001-.002L5.25 9.5l.53.53a.75.75 0 00.154-.838L3.822 4.5h.162c.305 0 .604-.08.869-.23l1.289-.737a.25.25 0 01.124-.033h.984V13h-2.5a.75.75 0 000 1.5h6.5a.75.75 0 000-1.5h-2.5V3.5h.984a.25.25 0 01.124.033l1.29.736c.264.152.563.231.868.231h.162l-2.112 4.692a.75.75 0 00.154.838l.53-.53-.53.53v.001l.002.002.002.002.006.006.016.015.045.04a3.517 3.517 0 00.686.45A4.492 4.492 0 0013 11c.88 0 1.556-.22 2.023-.454a3.512 3.512 0 00.686-.45l.045-.04.01-.01.006-.005.006-.006.002-.002.001-.002-.529-.531.53.53a.75.75 0 00.154-.838L13.823 4.5h.427a.75.75 0 000-1.5h-2.234a.25.25 0 01-.124-.033l-1.29-.736A1.75 1.75 0 009.735 2H8.75V.75zM1.695 9.227c.285.135.718.273 1.305.273s1.02-.138 1.305-.273L3 6.327l-1.305 2.9zm10 0c.285.135.718.273 1.305.273s1.02-.138 1.305-.273L13 6.327l-1.305 2.9z"/></svg>`,
	},
}

var kindAlert = ast.NewNodeKind("Alert")

type alertNode struct {
	ast.BaseBlock
	AlertType alertType
}

func (n *alertNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

func (n *alertNode) Kind() ast.NodeKind {
	return kindAlert
}

type alertTransformer struct{}

func (a *alertTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()

	type matchInfo struct {
		container ast.Node
		para      *ast.Paragraph
		info      alertType
		skipBytes int
	}
	var matches []matchInfo

	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		var p *ast.Paragraph
		var target ast.Node

		if bq, ok := n.(*ast.Blockquote); ok {
			target = bq
			for c := bq.FirstChild(); c != nil; c = c.NextSibling() {
				if cp, ok := c.(*ast.Paragraph); ok {
					p = cp
					break
				}
			}
		} else if para, ok := n.(*ast.Paragraph); ok {
			target = para
			p = para
		}

		if p == nil {
			return ast.WalkContinue, nil
		}

		pText := p.Text(source)
		raw := string(pText)
		trimmed := strings.TrimLeft(raw, " \t\n\r")
		if !strings.HasPrefix(trimmed, "[!") {
			return ast.WalkContinue, nil
		}

		idx := strings.Index(trimmed, "]")
		if idx == -1 {
			return ast.WalkContinue, nil
		}

		name := strings.ToUpper(trimmed[2:idx])
		info, ok := alertTypes[name]
		if !ok {
			return ast.WalkContinue, nil
		}

		skip := (len(raw) - len(trimmed)) + idx + 1
		matches = append(matches, matchInfo{
			container: target,
			para:      p,
			info:      info,
			skipBytes: skip,
		})

		return ast.WalkSkipChildren, nil
	})

	for _, m := range matches {
		rem := m.skipBytes
		// Strip the tag from text/string children
		for c := m.para.FirstChild(); c != nil && rem > 0; {
			next := c.NextSibling()
			if t, ok := c.(*ast.Text); ok {
				l := t.Segment.Len()
				if rem >= l {
					rem -= l
					m.para.RemoveChild(m.para, c)
				} else {
					start := t.Segment.Start + rem
					if start < t.Segment.Stop && source[start] == ' ' {
						start++
					}
					t.Segment = text.NewSegment(start, t.Segment.Stop)
					rem = 0
				}
			} else if s, ok := c.(*ast.String); ok {
				l := len(s.Value)
				if rem >= l {
					rem -= l
					m.para.RemoveChild(m.para, c)
				} else {
					s.Value = s.Value[rem:]
					rem = 0
				}
			} else {
				break
			}
			c = next
		}

		an := &alertNode{AlertType: m.info}
		parent := m.container.Parent()
		if parent != nil {
			parent.ReplaceChild(parent, m.container, an)
		}

		if _, ok := m.container.(*ast.Blockquote); ok {
			for c := m.container.FirstChild(); c != nil; {
				next := c.NextSibling()
				m.container.RemoveChild(m.container, c)
				an.AppendChild(an, c)
				c = next
			}
		} else {
			an.AppendChild(an, m.para)
		}
	}
}

type alertRenderer struct{}

func (r *alertRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindAlert, r.render)
}

func (r *alertRenderer) render(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*alertNode)
	if entering {
		_, _ = w.WriteString(fmt.Sprintf(`<div class="markdown-alert markdown-alert-%s"><!-- ALERT_NODE -->`, n.AlertType.kind))
		_, _ = w.WriteString(fmt.Sprintf(`<p class="markdown-alert-title">%s %s</p>`, n.AlertType.icon, n.AlertType.title))
	} else {
		_, _ = w.WriteString("</div>")
	}
	return ast.WalkContinue, nil
}

type alertExt struct{}

func (e *alertExt) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(&alertTransformer{}, 1),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&alertRenderer{}, 1),
	))
}

var AlertExtension = &alertExt{}
