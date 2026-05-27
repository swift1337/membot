package query

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenLParen
	tokenRParen
	tokenAND
	tokenOR
	tokenTERM
)

type token struct {
	kind tokenKind
	text string
}

type queryNode interface {
	queryNode()
}

type termNode struct {
	raw string
}

func (*termNode) queryNode() {}

type andNode struct {
	children []queryNode
}

func (*andNode) queryNode() {}

type orNode struct {
	children []queryNode
}

func (*orNode) queryNode() {}

type parser struct {
	tokens []token
	pos    int
}

func parseQuery(input string) (queryNode, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("query is empty")
	}

	tokens, err := lexQuery(input)
	if err != nil {
		return nil, err
	}

	p := &parser{tokens: tokens}
	node, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tokenEOF {
		return nil, fmt.Errorf("unexpected input after expression")
	}
	return node, nil
}

func lexQuery(input string) ([]token, error) {
	var tokens []token
	i := 0
	for i < len(input) {
		r, width := runeAt(input, i)
		if unicode.IsSpace(r) {
			i += width
			continue
		}

		switch r {
		case '(':
			tokens = append(tokens, token{kind: tokenLParen})
			i += width
			continue
		case ')':
			tokens = append(tokens, token{kind: tokenRParen})
			i += width
			continue
		}

		if _, ok := keywordAt(input, i, "AND"); ok {
			tokens = append(tokens, token{kind: tokenAND})
			i += len("AND")
			continue
		}
		if _, ok := keywordAt(input, i, "OR"); ok {
			tokens = append(tokens, token{kind: tokenOR})
			i += len("OR")
			continue
		}

		start := i
		for i < len(input) {
			r, w := runeAt(input, i)
			if unicode.IsSpace(r) || r == '(' || r == ')' {
				break
			}
			if _, ok := keywordAt(input, i, "AND"); ok {
				break
			}
			if _, ok := keywordAt(input, i, "OR"); ok {
				break
			}
			i += w
		}
		term := strings.TrimSpace(input[start:i])
		if term == "" {
			return nil, fmt.Errorf("invalid query syntax at position %d", start+1)
		}
		tokens = append(tokens, token{kind: tokenTERM, text: term})
	}

	tokens = append(tokens, token{kind: tokenEOF})
	return tokens, nil
}

func keywordAt(input string, i int, kw string) (bool, bool) {
	if !strings.HasPrefix(input[i:], kw) {
		return false, false
	}
	end := i + len(kw)
	if end < len(input) {
		next, _ := runeAt(input, end)
		if unicode.IsLetter(next) || unicode.IsDigit(next) || next == '_' {
			return false, false
		}
	}
	if i > 0 {
		prev, _ := runeAt(input, i-1)
		if unicode.IsLetter(prev) || unicode.IsDigit(prev) || prev == '_' {
			return false, false
		}
	}
	return true, true
}

func runeAt(s string, i int) (rune, int) {
	return utf8.DecodeRuneInString(s[i:])
}

func (p *parser) parseExpr() (queryNode, error) {
	return p.parseOrExpr()
}

func (p *parser) parseOrExpr() (queryNode, error) {
	left, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}

	var parts []queryNode
	parts = append(parts, left)

	for p.peek().kind == tokenOR {
		p.advance()
		right, err := p.parseAndExpr()
		if err != nil {
			return nil, err
		}
		parts = append(parts, right)
	}

	if len(parts) == 1 {
		return parts[0], nil
	}
	return &orNode{children: parts}, nil
}

func (p *parser) parseAndExpr() (queryNode, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	var parts []queryNode
	parts = append(parts, left)

	for {
		switch p.peek().kind {
		case tokenAND:
			p.advance()
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			parts = append(parts, right)
		case tokenTERM, tokenLParen:
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			parts = append(parts, right)
		default:
			if len(parts) == 1 {
				return parts[0], nil
			}
			return &andNode{children: parts}, nil
		}
	}
}

func (p *parser) parseUnary() (queryNode, error) {
	switch p.peek().kind {
	case tokenTERM:
		tok := p.advance()
		return &termNode{raw: tok.text}, nil
	case tokenLParen:
		p.advance()
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tokenRParen {
			return nil, fmt.Errorf("missing closing parenthesis")
		}
		p.advance()
		if inner == nil {
			return nil, fmt.Errorf("empty group in query")
		}
		return inner, nil
	case tokenOR, tokenAND:
		return nil, fmt.Errorf("unexpected operator %q", p.peek().textForError())
	case tokenRParen:
		return nil, fmt.Errorf("unexpected )")
	case tokenEOF:
		return nil, fmt.Errorf("unexpected end of query")
	default:
		return nil, fmt.Errorf("invalid query syntax")
	}
}

func (p *parser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{kind: tokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) advance() token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (t token) textForError() string {
	switch t.kind {
	case tokenAND:
		return "AND"
	case tokenOR:
		return "OR"
	default:
		return t.text
	}
}
