package schemalex

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// eof is returned by the lexer at the end of the input.
// It must not be a valid rune, so that a NUL character in the input is not mistaken for the end of the input.
const eof = rune(-1)

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

	// inExecutableComment is true while the lexer is in an executable comment.
	inExecutableComment bool

	// executableCommentOpener is the token that opens the last executable comment.
	executableCommentOpener *Token

	// executableCommentMarkers is the set of the tokens that open or close executable comments.
	executableCommentMarkers map[*Token]struct{}
}

func lex(input []byte) []*Token {
	l := newLexer(input)
	l.run()
	return l.out
}

// emitExecutableCommentMarker emits a COMMENT_IDENT token that opens or closes an executable comment.
func (l *lexer) emitExecutableCommentMarker() {
	l.emit(COMMENT_IDENT)
	if l.executableCommentMarkers == nil {
		l.executableCommentMarkers = make(map[*Token]struct{})
	}
	l.executableCommentMarkers[l.out[len(l.out)-1]] = struct{}{}
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
				l.advance()
				if !l.inExecutableComment && l.peek() == '!' {
					// an executable comment: `/*! ... */` or `/*!NNNNN ... */`
					// its content is lexed as SQL.
					l.advance()
					l.skipVersionNumber()
					l.inExecutableComment = true
					l.emitExecutableCommentMarker()
					l.executableCommentOpener = l.out[len(l.out)-1]
					continue OUTER
				}
				l.runCComment()
				l.emit(COMMENT_IDENT)
			default:
				l.emit(SLASH)
			}
		case '*':
			if l.inExecutableComment && l.peek() == '/' {
				// the end of the executable comment
				l.advance()
				l.inExecutableComment = false
				l.emitExecutableCommentMarker()
				continue OUTER
			}
			l.emit(ILLEGAL)
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

// skipVersionNumber skips the version number of an executable comment.
// The version number is 5 or 6 digits, e.g. `/*!80000 ... */` or `/*!100000 ... */`.
// As MySQL does, 6 digits are a version number only if they are followed by a whitespace,
// otherwise the first 5 digits are the version number.
// Less than 5 digits are not a version number, but a part of the content.
// https://dev.mysql.com/doc/refman/26.10/en/comments.html
func (l *lexer) skipVersionNumber() {
	// the version number is ASCII, so we can look ahead the input bytes directly.
	// there must be no read-ahead runes here.
	rest := l.input[l.cur.pos:]
	n := 0
	for n < 6 && n < len(rest) && rest[n] >= '0' && rest[n] <= '9' {
		n++
	}
	switch {
	case n < 5:
		return
	case n == 6 && (len(rest) == 6 || !isSpace(rune(rest[6]))):
		n = 5
	}
	for range n {
		l.advance()
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
