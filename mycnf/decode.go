package mycnf

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

func Unmarshal(cnf []byte) (MyCnf, error) {
	p := &parser{
		cnf:    cnf,
		result: MyCnf{},
	}
	p.Parse()
	if p.err != nil {
		return nil, p.err
	}
	return p.result, nil
}

func isWhitespace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ':
		return true
	}
	return false
}

func isWhitespaceWithoutEOL(r rune) bool {
	switch r {
	case '\t', '\v', '\f', ' ':
		return true
	}
	return false
}

const eof = -1

type parser struct {
	cnf    []byte
	pos    int
	result MyCnf

	group string // current group
	err   error
}

func (p *parser) Parse() {
	for {
		p.SkipWhiteSpaces()
		ch := p.Peek()
		switch ch {
		case eof:
			return
		case '#', ';':
			// comment, skip this line
			for {
				ch := p.Read()
				if ch == eof || ch == '\n' {
					break
				}
			}
		case '[':
			// [group]
			p.ParseGroup()
		default:
			// opt_name=value
			p.ParseOption()
		}
		if p.err != nil {
			return
		}
	}
}

func (p *parser) Peek() rune {
	if p.pos >= len(p.cnf) {
		return eof
	}
	r, _ := utf8.DecodeRune(p.cnf[p.pos:])
	return r
}

func (p *parser) Read() rune {
	if p.pos >= len(p.cnf) {
		return eof
	}
	r, size := utf8.DecodeRune(p.cnf[p.pos:])
	p.pos += size
	return r
}

func (p *parser) SkipWhiteSpaces() {
	for isWhitespace(p.Peek()) {
		p.Read()
	}
}

func (p *parser) SkipWhiteSpacesWithoutEOL() {
	for isWhitespaceWithoutEOL(p.Peek()) {
		p.Read()
	}
}

func (p *parser) SkipToEOL() {
	p.SkipWhiteSpacesWithoutEOL()
	ch := p.Peek()
	switch ch {
	case '#':
		// comment
		for {
			ch := p.Read()
			if ch == eof || ch == '\n' {
				break
			}
		}
	case '\r', '\n', eof:
		// EOL
	default:
		panic(fmt.Errorf("unexpected charactor: %c", ch))
	}
}

// `[group]`
func (p *parser) ParseGroup() {
	if ch := p.Read(); ch != '[' {
		panic(fmt.Errorf("unexpected section start: %c", ch))
	}

	var buf strings.Builder
LOOP:
	for {
		ch := p.Peek()
		switch ch {
		case ']':
			p.Read()
			break LOOP
		case '\n', '\r':
			p.err = errors.New("unexpected new line")
			return
		case eof:
			p.err = errors.New("unexpected EOF")
			return
		default:
			p.Read()
			buf.WriteRune(ch)
		}
	}
	p.group = buf.String()
}

func (p *parser) ParseOption() {
	name := p.ParseOptionName()
	value := p.ParseOptionValue()
	p.SkipToEOL()
	if g, ok := p.result[p.group]; ok {
		g[name] = value
	} else {
		p.result[p.group] = map[string]string{
			name: value,
		}
	}
}

func (p *parser) ParseOptionName() string {
	var buf strings.Builder
LOOP:
	for {
		ch := p.Peek()
		switch ch {
		case '=', '\r', '\n', eof:
			break LOOP
		}
		buf.WriteRune(unicode.ToLower(ch))
		p.Read()
	}
	return buf.String()
}

func (p *parser) ParseOptionValue() string {
	if p.Peek() != '=' {
		return ""
	}
	p.Read()

	p.SkipWhiteSpacesWithoutEOL()
	switch p.Peek() {
	case '"':
		p.Read() // skip first '"'
		// read until correspond '"'
		var buf strings.Builder
		for {
			ch := p.Peek()
			if ch == '"' {
				p.Read()
				break
			}
			if ch == '\r' || ch == '\n' || ch == eof {
				p.err = errors.New("unexpected EOL")
				return ""
			}
			buf.WriteRune(p.ParseEscapedRune())
		}
		p.SkipToEOL()
		return buf.String()
	case '\'':
		p.Read() // skip first '\''
		// read until correspond '\''
		var buf strings.Builder
		for {
			ch := p.Peek()
			if ch == '\'' {
				p.Read()
				break
			}
			if ch == '\r' || ch == '\n' || ch == eof {
				p.err = errors.New("unexpected EOL")
				return ""
			}
			buf.WriteRune(p.ParseEscapedRune())
		}
		p.SkipToEOL()
		return buf.String()
	default:
		// read the value until EOL
		var buf strings.Builder
		for {
			ch := p.Peek()
			if ch == '\r' || ch == '\n' || ch == eof {
				break
			}
			buf.WriteRune(p.ParseEscapedRune())
		}
		return strings.TrimRightFunc(buf.String(), isWhitespace)
	}
}

func (p *parser) ParseEscapedRune() rune {
	ch := p.Read()
	if ch != '\\' {
		return ch
	}
	ch = p.Peek()
	switch ch {
	case 'n':
		ch = '\n'
	case 'r':
		ch = '\r'
	case 't':
		ch = '\t'
	case 'b':
		ch = '\b'
	case 's':
		ch = ' '
	case '"':
		ch = '"'
	case '\'':
		ch = '\''
	case '\\':
		ch = '\\'
	default:
		// unknown escape sequence
		// MySQL leaves '\\'
		return '\\'
	}
	p.Read()
	return ch
}
