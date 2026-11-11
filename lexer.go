package schemalex

import (
	"errors"
	"strings"
	"unicode/utf8"
)

const eof = rune(0)

type lrune struct {
	r rune
	w int
}

type position struct {
	pos  int // byte count
	col  int
	line int
}

type lexer struct {
	out       []*Token
	input     []byte
	peekCount int
	peekRunes [3]lrune

	start position // position where we last emitted
	cur   position // current position including read-ahead
	width int
}

func lex(input []byte) []*Token {
	l := newLexer(input)
	l.run()
	return l.out
}

func newLexer(input []byte) *lexer {
	var l lexer
	l.input = input
	l.start.line = 1
	l.start.col = 1
	l.cur.line = 1
	l.cur.col = 1
	l.peekCount = -1
	return &l
}

func (l *lexer) emit(typ TokenType) {
	var t Token
	t.Line = l.start.line
	t.Col = l.start.col
	t.Type = typ
	t.Pos = l.start.pos

	if typ == EOF {
		t.EOF = true
		t.Pos = len(l.input)
	} else {
		t.Value = l.str()
		switch typ {
		case SINGLE_QUOTE_IDENT:
			t.Value = unescapeQuotes(t.Value, '\'', true)
		case DOUBLE_QUOTE_IDENT:
			t.Value = unescapeQuotes(t.Value, '"', true)
		case BACKTICK_IDENT:
			t.Value = unescapeQuotes(t.Value, '`', false)
		}
	}

	l.out = append(l.out, &t)

	// when we emit, we must copy the value of cur to start
	// but we also must adjust the position by the read-ahead offset
	l.start = l.cur
	l.start.pos = l.start.pos - (l.peekCount + 1)
}

func (l *lexer) str() string {
	endpos := l.cur.pos - (l.peekCount + 1)
	w := len(l.input[l.start.pos:])
	if endpos-l.start.pos > w {
		endpos = l.start.pos + w
	}
	return string(l.input[l.start.pos:endpos])
}

func (l *lexer) run() {
OUTER:
	for {
		r := l.peek()

		// These require peek, and then consume
		switch {
		case isSpace(r):
			// read until space end
			l.runSpace()
			l.emit(SPACE)
			continue OUTER
		case isLetter(r):
			t := l.runIdent()
			s := l.str()
			if typ, ok := keywordIdentMap[strings.ToUpper(s)]; ok {
				t = typ
			}
			l.emit(t)
			continue OUTER
		case isDigit(r):
			l.runNumber()
			l.emit(NUMBER)
			continue OUTER
		}

		// once we got here, we can consume
		l.advance()
		switch r {
		case eof:
			l.emit(EOF)
			return
		case '`':
			if err := l.runQuote('`', false); err != nil {
				l.emit(ILLEGAL)
				return
			}

			l.emit(BACKTICK_IDENT)
		case '"':
			if err := l.runQuote('"', true); err != nil {
				l.emit(ILLEGAL)
				return
			}

			l.emit(DOUBLE_QUOTE_IDENT)
		case '\'':
			if err := l.runQuote('\'', true); err != nil {
				l.emit(ILLEGAL)
				return
			}

			l.emit(SINGLE_QUOTE_IDENT)
		case '/':
			switch c := l.peek(); c {
			case '*':
				l.runCComment()
				l.emit(COMMENT_IDENT)
			default:
				l.emit(SLASH)
			}
		case '-':
			switch r1 := l.peek(); {
			case r1 == '-':
				l.advance()
				// TODO: https://dev.mysql.com/doc/refman/5.6/en/comments.html
				// TODO: not only space. control character
				if !isSpace(l.peek()) {
					l.emit(DASH)
					continue OUTER
				}
				l.runToEOL()
				l.emit(COMMENT_IDENT)
			case isDigit(r1):
				l.runNumber()
				l.emit(NUMBER)
			default:
				l.emit(DASH)
			}
		case '#':
			// https://dev.mysql.com/doc/refman/5.6/en/comments.html
			l.runToEOL()
			l.emit(COMMENT_IDENT)
		case '(':
			l.emit(LPAREN)
		case ')':
			l.emit(RPAREN)
		case ';':
			l.emit(SEMICOLON)
		case ',':
			l.emit(COMMA)
		case '.':
			if isDigit(l.peek()) {
				l.runNumber()
				l.emit(NUMBER)
			} else {
				l.emit(DOT)
			}
		case '+':
			if isDigit(l.peek()) {
				l.runNumber()
				l.emit(NUMBER)
			} else {
				l.emit(PLUS)
			}
		case '=':
			l.emit(EQUAL)
		default:
			l.emit(ILLEGAL)
		}
	}
}

func (l *lexer) next() rune {
	r := l.peek()
	l.advance()
	return r
}

func (l *lexer) peek() rune {
	if l.peekCount >= 0 {
		return l.peekRunes[l.peekCount].r
	}

	if l.cur.pos >= len(l.input) {
		l.width = 0
		return eof
	}

	r, w := utf8.DecodeRune(l.input[l.cur.pos:])
	l.peekCount++
	l.peekRunes[l.peekCount].r = r
	l.peekRunes[l.peekCount].w = w
	l.cur.pos += w

	return r
}

func (l *lexer) advance() {
	// if the current rune is a new line, we line++
	r := l.peek()
	switch r {
	case '\n':
		l.cur.line++
		l.cur.col = 0
	case eof:
	default:
		l.cur.col++
	}

	l.peekCount--
}

func (l *lexer) runSpace() {
	for isSpace(l.peek()) {
		l.advance()
	}
}

func (l *lexer) runIdent() TokenType {
OUTER:
	for {
		r := l.peek()
		switch {
		case r == eof:
			l.advance()
			break OUTER
		case isCharacter(r):
			l.advance()
		default:
			break OUTER
		}
	}
	return IDENT
}

// unescapeQuotes removes the surrounding quotes from s and unescapes the quotes in it.
// A doubled quote is unescaped to a single quote.
// If backslash is true, a backslash escapes the next character:
// a backslash-escaped quote is unescaped, and the other escape sequences are kept as they are.
func unescapeQuotes(s string, quot rune, backslash bool) string {
	// assume the first rune and the last rune are quot
	w := utf8.RuneLen(quot)
	s = s[w : len(s)-w]

	var buf strings.Builder
	buf.Grow(len(s))
	rdr := strings.NewReader(s)
	for {
		r, _, err := rdr.ReadRune()
		if err != nil {
			break
		}
		switch {
		case r == quot:
			// a doubled quote: skip the second one.
			// the lexer guarantees that it exists.
			_, _, _ = rdr.ReadRune()
		case r == '\\' && backslash:
			r2, _, err := rdr.ReadRune()
			if err != nil {
				break
			}
			if r2 == quot {
				r = quot
			} else {
				buf.WriteRune(r)
				r = r2
			}
		}
		buf.WriteRune(r)
	}
	return buf.String()
}

// runQuote reads a quoted string or identifier until the closing quote.
// If backslash is true, a backslash escapes the next character.
// https://dev.mysql.com/doc/refman/26.10/en/string-literals.html
// https://dev.mysql.com/doc/refman/26.10/en/identifiers.html
func (l *lexer) runQuote(pair rune, backslash bool) error {
	for {
		r := l.next()
		if r == eof {
			return errors.New(`unexpected eof`)
		} else if r == '\\' && backslash {
			if l.next() == eof {
				return errors.New(`unexpected eof`)
			}
		} else if r == pair {
			if l.peek() == pair {
				// it is escape
				l.next()
			} else {
				return nil
			}
		}
	}
}

// https://dev.mysql.com/doc/refman/5.6/en/comments.html
func (l *lexer) runCComment() {
	for {
		r := l.next()
		switch r {
		case eof:
			return
		case '*':
			if l.peek() == '/' {
				l.advance()
				return
			}
		}
	}
}

func (l *lexer) runToEOL() TokenType {
	for {
		r := l.next()
		switch r {
		case eof, '\n':
			return COMMENT_IDENT
		}
	}
}

// https://dev.mysql.com/doc/refman/5.6/en/number-literals.html
func (l *lexer) runDigit() {
	for {
		if !isDigit(l.peek()) {
			break
		}
		l.advance()
	}
}

func (l *lexer) runNumber() {
	l.runDigit()
	if l.peek() == '.' {
		l.advance()
		l.runDigit()
	}

	switch l.peek() {
	case 'E', 'e':
		l.advance()
		if l.peek() == '-' {
			l.advance()
		}
		l.runDigit()
	}
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\n' || r == '\t' || r == '\r'
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isCharacter(r rune) bool {
	return isDigit(r) || isLetter(r) || r == '_'
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
